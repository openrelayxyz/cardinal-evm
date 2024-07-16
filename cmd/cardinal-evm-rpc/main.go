package main

import (
	"math/big"
	"net"
	_ "net/http/pprof"
	"net/http"
	"flag"
	"fmt"
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/api"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/txemitter"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/streams"
	"github.com/openrelayxyz/cardinal-types/metrics"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-rpc/transports"
	"github.com/openrelayxyz/cardinal-streams/delivery"
	"github.com/openrelayxyz/cardinal-storage/resolver"
	"github.com/savaki/cloudmetrics"
	"github.com/pubnub/go-metrics-statsd"
	"strconv"
	"time"
	"os"
)

func main() {
	resumptionTime := flag.Int64("resumption.ts", -1, "Resume from a timestamp instead of the offset committed to the database")
	blockRollback := flag.Int64("block.rollback", 0, "Rollback to block N before syncing. If N < 0, rolls back from head before starting or syncing.")
	exitWhenSynced := flag.Bool("exitwhensynced", false, "Automatically shutdown after syncing is complete")
	initArchive := flag.Bool("init.archive", false, "When initializing from genesis, should this be an archival database?")
	genesisJson := flag.String("init.genesis", "", "File containing genesis block JSON for database initialization")
	shanghaiBlock := flag.Int64("shanghai.block", -1, "Override shanghai hardfork time")
	debug := flag.Bool("debug", false, "Enable debug APIs")

	flag.CommandLine.Parse(os.Args[1:])
	cfg, err := LoadConfig(flag.CommandLine.Args()[0])
	if err != nil {
		log.Error("Error parsing config", "err", err)
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

	if *genesisJson != "" {
		if err := genesisInit(cfg.DataDir, *genesisJson, *initArchive); err != nil {
			log.Error("Error initializing", "err", err)
			os.Exit(1)
		}
		log.Info("Initialization completed")
	}

	// TODO: Once Cardinal streams supports it, pass multiple brokers into the stream
	broker := cfg.Brokers[0]

	heightCh := make(chan *rpc.HeightRecord, 1024)

	tm := transports.NewTransportManager(cfg.Concurrency)
	if cfg.HttpPort != 0 {
		tm.AddHTTPServer(cfg.HttpPort)
	}
	if cfg.BlockWaitTime > 0 {
		tm.SetBlockWaitDuration(time.Duration(cfg.BlockWaitTime) * time.Millisecond)
	}
	tm.RegisterHeightFeed(heightCh)
	s, err := resolver.ResolveStorage(cfg.DataDir, cfg.ReorgThreshold, cfg.whitelist)
	if err != nil {
		log.Error("Error opening current storage", "error", err, "datadir", cfg.DataDir)
		os.Exit(1)
	}
	if *blockRollback < 0 {
		_, n, _, _ := s.LatestBlock()
		*blockRollback = int64(n) + *blockRollback
	}
	if *blockRollback != 0 {
		if err := s.Rollback(uint64(*blockRollback)); err != nil {
			log.Error("Rollback error", "err", err)
			s.Close()
			os.Exit(1)
		}
	}
	chaincfg, ok := params.ChainLookup[cfg.Chainid]
	if !ok {
		log.Error("Unsupported chainid", "chain", cfg.Chainid)
		s.Close()
		os.Exit(1)
	}
	if *shanghaiBlock >= 0 {
		chaincfg.ShanghaiBlock = big.NewInt(*shanghaiBlock)
	}
	sm, err := streams.NewStreamManager(
		cfg.brokers,
		cfg.ReorgThreshold,
		cfg.Chainid,
		s,
		cfg.whitelist,
		*resumptionTime,
		heightCh,
	)
	if err != nil {
		log.Error("Error connecting streams", "err", err)
		os.Exit(1)
	}
	s.RegisterWaiter(sm.Waiter(), time.Duration(cfg.BlockWaitTime) * time.Millisecond)
	mgr := vm.NewEVMManager(s, cfg.Chainid, vm.Config{}, chaincfg)
	tm.Register("eth", api.NewETHAPI(s, mgr, cfg.Chainid, cfg.GasLimitOpts.RPCGasLimit()))
	tm.Register("ethercattle", api.NewEtherCattleBlockChainAPI(mgr, cfg.GasLimitOpts.RPCGasLimit()))
	tm.Register("web3", &api.Web3API{})
	tm.Register("net", &api.NetAPI{chaincfg.NetworkID})
	if broker.URL != "" && cfg.TransactionTopic != "" {
		emitter, err := txemitter.NewKafkaTransactionProducerFromURLs(broker.URL, cfg.TransactionTopic)
		if err != nil {
			log.Error("Error setting up transaction producer", "error", err, "broker", broker.URL, "topic", cfg.TransactionTopic)
			os.Exit(1)
		}
		tm.Register("eth", api.NewPublicTransactionPoolAPI(emitter, mgr))
	}
	if *debug {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()
		tm.Register("debug", sm.API())
		tm.Register("debug", api.NewDebugAPI(s, mgr, cfg.Chainid))
		tm.Register("debug", &metrics.MetricsAPI{})
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
		log.Info("--exitwhensynced set: shutting down")
		sm.Close()
		s.Vacuum(cfg.RollbackThreshold, time.Duration(cfg.VacuumTime) * time.Second)
		s.Close()
		if sm.Processed() == 0 {
			log.Info("Shutting down without processing any messages.")
			os.Exit(3)
		}
		os.Exit(0)
	}
	metrics.Clear()
	delivery.Ready()

	if cfg.Statsd != nil && cfg.Statsd.Port != "" {
		addr := "127.0.0.1:" + cfg.Statsd.Port
		if cfg.Statsd.Address != "" {
			addr = fmt.Sprintf("%v:%v", cfg.Statsd.Address, cfg.Statsd.Port)
		}
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			log.Error("Invalid Address. Statsd will not be configured.", "error", err.Error())
		} else {
			interval := time.Duration(cfg.Statsd.Interval) * time.Second
			if cfg.Statsd.Interval == 0 {
				interval = time.Second
			}
			prefix := cfg.Statsd.Prefix
			if prefix == "" {
				prefix = "cardinal.evm"
			}
			go statsd.StatsD(
				metrics.MajorRegistry,
				interval,
				prefix,
				udpAddr,
			)
			if cfg.Statsd.Minor {
				go statsd.StatsD(
					metrics.MinorRegistry,
					interval,
					prefix,
					udpAddr,
				)
			}
		}
	}
	if cfg.CloudWatch != nil {
		namespace := cfg.CloudWatch.Namespace
		if namespace == "" {
			namespace = "Cardinal"
		}
		dimensions := []string{}
		for k, v := range cfg.CloudWatch.Dimensions {
			dimensions = append(dimensions, k, v)
		}
		if len(dimensions) == 0 {
			dimensions = append(dimensions, "chainid", strconv.Itoa(int(cfg.Chainid)))
		}
		cwcfg := []func(*cloudmetrics.Publisher){
			cloudmetrics.Dimensions(dimensions...),
		}
		if cfg.CloudWatch.Interval > 0 {
			cwcfg = append(cwcfg, cloudmetrics.Interval(time.Duration(cfg.CloudWatch.Interval) * time.Second))
		}
		if len(cfg.CloudWatch.Percentiles) > 0 {
			cwcfg = append(cwcfg, cloudmetrics.Percentiles(cfg.CloudWatch.Percentiles))
		}
		go cloudmetrics.Publish(metrics.MajorRegistry,
			namespace,
			cwcfg...
		)
		if cfg.CloudWatch.Minor {
			go cloudmetrics.Publish(metrics.MinorRegistry,
				namespace,
				cwcfg...
			)
		}
	}

	tm.RegisterHealthCheck(cfg.HealthChecks)
	tm.RegisterHealthCheck(sm)
	cfg.HealthChecks.Start(tm.Caller())
	if err := tm.Run(cfg.HealthCheckPort); err != nil {
		log.Error("Critical Error. Shutting down.", "error", err)
		sm.Close()
		s.Close()
		os.Exit(1)
	}
	tm.Stop()
	sm.Close()
	s.Close()
}
