package main

import (
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
	"github.com/openrelayxyz/cardinal-rpc/transports"
	"github.com/openrelayxyz/cardinal-storage/current"
	"github.com/openrelayxyz/cardinal-storage/db/badgerdb"
	"github.com/openrelayxyz/cardinal-types"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	home, _ := os.UserHomeDir()
	port := flag.Int64("http.port", 8000, "HTTP port")
	concurrency := flag.Int("concurrency", runtime.NumCPU()*4, "How many concurrent requests to support (additional requests will be queued)")
	chainid := flag.Int64("chainid", 1, "The web3 chainid for this network")
	reorgThreshold := flag.Int64("reorg.threshold", 128, "Number of blocks to be able to support for quick reorgs")
	dataDir := flag.String("datadir", "", "Directory for Cardinal data and configuration")
	rollback := flag.Int64("rollback", 5000, "The number of messages to roll back when resuming message processing")
	transactionTopic := flag.String("topic.transactions", "", "The topic for broadcasting transactions to the network")
	whitelistString := flag.String("whitelist", "", "A comma separated list mapping block numbers to specific hashes. eg. 1234=0x1a03...8cbf - Used to force Cardinal to the right side of a chain split, even if the wrong side is heavier")
	exitWhenSynced := flag.Bool("exitwhensynced", false, "Automatically shutdown after syncing is complete")
	debug := flag.Bool("debug", false, "Enable debug APIs")
	logLevel := flag.Int("log.level", 2, "Specify the log level. 1 = debug, 2 = info, 3 = warn, 4 = err, 5 = crit")

	defaultTopic := flag.String("default.topic", "", "Default topic for Cardinal broker")
	blockTopic := flag.String("block.topic", "", "Topic for Cardinal block data")
	codeTopic := flag.String("code.topic", "", "Topic for Cardinal contract code")
	stateTopic := flag.String("state.topic", "", "Topic for Cardinal state data")

	flag.CommandLine.Parse(os.Args[1:])

	if *dataDir == "" {
		*dataDir = path.Join(home, ".cardinal", "evm", strconv.Itoa(int(*chainid)))
	}

	var logLvl log.Lvl
	switch *logLevel {
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

	brokerURL := flag.CommandLine.Args()[0]
	if *defaultTopic == "" { *defaultTopic = fmt.Sprintf("cardinal-%v", *chainid) }
	if *blockTopic == "" { *blockTopic = fmt.Sprintf("%v-block", *defaultTopic) }
	if *codeTopic == "" { *codeTopic = fmt.Sprintf("%v-code", *defaultTopic) }
	if *stateTopic == "" { *stateTopic = fmt.Sprintf("%v-state", *defaultTopic) }

	whitelist := make(map[uint64]types.Hash)
	for _, entry := range strings.Split(*whitelistString, ",") {
		parts := strings.Split(entry, "=")
		if len(parts) != 2 { continue }
		blockNum, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Error("Error parsing whitelist flag", "err", err)
			os.Exit(1)
		}
		whitelist[uint64(blockNum)] = types.HexToHash(parts[1])
	}

	tm := transports.NewTransportManager(*concurrency)
	tm.AddHTTPServer(*port)
	db, err := badgerdb.New(*dataDir)
	if err != nil {
		log.Error("Error opening badgerdb", "error", err)
	}
	s, err := current.Open(db, *reorgThreshold, whitelist)
	if err != nil {
		log.Error("Error opening current storage", "error", err, "datadir", *dataDir)
		os.Exit(1)
	}
	chaincfg, ok := params.ChainLookup[*chainid]
	if !ok {
		log.Error("Unsupported chainid", "chain", chainid)
		os.Exit(1)
	}
	sm, err := streams.NewStreamManager(
		brokerURL,
		*defaultTopic,
		[]string{*defaultTopic, *blockTopic, *codeTopic, *stateTopic},
		*rollback,
		*reorgThreshold,
		*chainid,
		s,
		whitelist,
	)
	if err != nil {
		log.Error("Error connecting streams", "err", err)
		os.Exit(1)
	}
	mgr := vm.NewEVMManager(s, *chainid, vm.Config{}, chaincfg)
	tm.Register("eth", api.NewETHAPI(s, mgr, *chainid))
	tm.Register("ethercattle", api.NewEtherCattleBlockChainAPI(mgr))
	tm.Register("web3", &api.Web3API{})
	tm.Register("net", &api.NetAPI{chaincfg.NetworkID})
	if 	brokerURL != "" && *transactionTopic != "" {
		emitter, err := txemitter.NewKafkaTransactionProducerFromURLs(brokerURL, *transactionTopic)
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
	if err := tm.Run(); err != nil {
		log.Error("Critical Error. Shutting down.", "error", err)
		sm.Close()
		s.Close()
		db.Close()
		os.Exit(1)
	}
}
