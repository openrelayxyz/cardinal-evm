package main

import (
	"flag"
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/api"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/txemitter"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc/transports"
	"github.com/openrelayxyz/cardinal-storage/current"
	"github.com/openrelayxyz/cardinal-storage/db/badgerdb"
	"os"
	"path"
	"runtime"
)

func main() {
	home, _ := os.UserHomeDir()
	port := flag.Int64("http.port", 8000, "HTTP port")
	concurrency := flag.Int("concurrency", runtime.NumCPU()*4, "How many concurrent requests to support (additional requests will be queued)")
	reorgThreshold := flag.Int64("reorg.threshold", 128, "Number of blocks to be able to support for quick reorgs")
	dataDir := flag.String("datadir", path.Join(home, ".cardinal", "evm"), "Directory for Cardinal data and configuration")
	chainid := flag.Int64("chainid", 1, "The web3 chainid for this network")
	brokerURL := flag.String("message.broker", "", "A message broker supported by Cardinal Streams")
	transactionTopic := flag.String("topic.transactions", "", "The topic for broadcasting transactions to the network")
	flag.CommandLine.Parse(os.Args[1:])

	tm := transports.NewTransportManager(*concurrency)
	tm.AddHTTPServer(*port)
	db, err := badgerdb.New(*dataDir)
	if err != nil {
		log.Error("Error opening badgerdb", "error", err)
	}
	s, err := current.Open(db, *reorgThreshold)
	if err != nil {
		log.Error("Error opening current storage", "error", err)
		os.Exit(1)
	}
	chaincfg, ok := params.ChainLookup[*chainid]
	if !ok {
		log.Error("Unsupported chainid", "chain", chainid)
		os.Exit(1)
	}
	mgr := vm.NewEVMManager(s, *chainid, vm.Config{}, chaincfg)
	tm.Register("eth", api.NewETHAPI(s, mgr, *chainid))
	tm.Register("ethercattle", api.NewEtherCattleBlockChainAPI(mgr))
	tm.Register("web3", &api.Web3API{})
	tm.Register("net", &api.NetAPI{chaincfg.NetworkID})
	if *brokerURL != "" && *transactionTopic != "" {
		emitter, err := txemitter.NewKafkaTransactionProducerFromURLs(*brokerURL, *transactionTopic)
		if err != nil {
			log.Error("Error setting up transaction producer", "error", err)
			os.Exit(1)
		}
		tm.Register("eth", api.NewPublicTransactionPoolAPI(emitter, mgr))
	}
	if err := tm.Run(); err != nil {
		log.Error("Critical Error. Shutting down.", "error", err)
		db.Close()
		os.Exit(1)
	}
}
