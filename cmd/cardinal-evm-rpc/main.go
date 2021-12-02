package main

import (
	_ "net/http/pprof"
	"net/http"
	"flag"
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/api"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/txemitter"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/streams"
	"github.com/openrelayxyz/cardinal-rpc/transports"
	"github.com/openrelayxyz/cardinal-storage/current"
	"github.com/openrelayxyz/cardinal-storage/db/badgerdb"
	"os"
)

func main() {
	exitWhenSynced := flag.Bool("exitwhensynced", false, "Automatically shutdown after syncing is complete")
	debug := flag.Bool("debug", false, "Enable debug APIs")

	flag.CommandLine.Parse(os.Args[1:])
	cfg, err := LoadConfig(flag.CommandLine.Args()[0])
	if err != nil {
		log.Error("Error parsing whitelist flag", "err", err)
		os.Exit(1)
	}

	var logLvl log.Lvl
	switch cfg.LogLevel {
	case 1:
		logLvl = log.LvlDebug
	case 2:
		logLvl = log.LvlInfo
	case 3:
		logLvl = log.LvlWarn
	case 4:
		logLvl = log.LvlError
	case 5:
		logLvl = log.LvlCrit
	default:
		logLvl = log.LvlInfo
	}
	log.Root().SetHandler(log.LvlFilterHandler(logLvl, log.Root().GetHandler()))

	if len(cfg.Brokers) == 0 {
		log.Error("No brokers specified")
		os.Exit(1)
	}

	// TODO: Once Cardinal streams supports it, pass multiple brokers into the stream
	broker := cfg.Brokers[0]

	tm := transports.NewTransportManager(cfg.Concurrency)
	if cfg.HttpPort != 0 {
		tm.AddHTTPServer(cfg.HttpPort)
	}
	db, err := badgerdb.New(cfg.DataDir)
	if err != nil {
		log.Error("Error opening badgerdb", "error", err)
	}
	s, err := current.Open(db, cfg.ReorgThreshold, cfg.Whitelist)
	if err != nil {
		log.Error("Error opening current storage", "error", err, "datadir", cfg.DataDir)
		os.Exit(1)
	}
	chaincfg, ok := params.ChainLookup[cfg.Chainid]
	if !ok {
		log.Error("Unsupported chainid", "chain", cfg.Chainid)
		os.Exit(1)
	}
	sm, err := streams.NewStreamManager(
		cfg.brokers,
		cfg.ReorgThreshold,
		cfg.Chainid,
		s,
		cfg.Whitelist,
	)
	if err != nil {
		log.Error("Error connecting streams", "err", err)
		os.Exit(1)
	}
	mgr := vm.NewEVMManager(s, cfg.Chainid, vm.Config{}, chaincfg)
	tm.Register("eth", api.NewETHAPI(s, mgr, cfg.Chainid))
	tm.Register("ethercattle", api.NewEtherCattleBlockChainAPI(mgr))
	tm.Register("web3", &api.Web3API{})
	tm.Register("net", &api.NetAPI{chaincfg.NetworkID})
	if broker.URL != "" && cfg.TransactionTopic != "" {
		emitter, err := txemitter.NewKafkaTransactionProducerFromURLs(broker.URL, cfg.TransactionTopic)
		if err != nil {
			log.Error("Error setting up transaction producer", "error", err)
			os.Exit(1)
		}
		tm.Register("eth", api.NewPublicTransactionPoolAPI(emitter, mgr))
	}
	if *debug {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()
		tm.Register("debug", sm.API())
	}
	log.Debug("Starting stream")
	if err := sm.Start(); err != nil {
		log.Error("Error starting stream", "error", err)
		os.Exit(1)
	}
	log.Debug("Waiting for stream to be ready")
	<-sm.Ready()
	log.Debug("Stream ready")
	if *exitWhenSynced {
		sm.Close()
		s.Close()
		db.Close()
		os.Exit(0)
	}
	tm.RegisterHealthCheck(cfg.HealthChecks)
	cfg.HealthChecks.Start(tm.Caller())
	if err := tm.Run(cfg.HealthCheckPort); err != nil {
		log.Error("Critical Error. Shutting down.", "error", err)
		sm.Close()
		s.Close()
		db.Close()
		os.Exit(1)
	}
	tm.Stop()
	sm.Close()
	s.Close()
	db.Close()
}
