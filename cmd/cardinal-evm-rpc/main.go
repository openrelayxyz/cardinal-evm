package main

import (
  log "github.com/inconshreveable/log15"
  "github.com/openrelayxyz/cardinal-evm/api"
  "github.com/openrelayxyz/cardinal-evm/vm"
  "github.com/openrelayxyz/cardinal-evm/params"
  "github.com/openrelayxyz/cardinal-storage/current"
  "github.com/openrelayxyz/cardinal-storage/db/badgerdb"
  "github.com/openrelayxyz/cardinal-rpc/transports"
  "os"
  "strconv"
)

func main() {
  tm := transports.NewTransportManager(10)
  tm.AddHTTPServer(8000)
  db, err := badgerdb.New(os.Args[1])
  if err != nil {
    log.Error("Error opening badgerdb", "error", err)
  }
  s, err  := current.Open(db, 128)
  if err != nil {
    log.Error("Error opening current storage", "error", err)
    os.Exit(1)
  }
  chainid, err := strconv.Atoi(os.Args[2])
  if err != nil {
    log.Error("Invalid argument 2. Expected int.", "error", err )
    os.Exit(1)
  }
  chaincfg, ok := params.ChainLookup[int64(chainid)]
  if !ok {
    log.Error("Unsupported chainid", "chain", chainid)
    os.Exit(1)
  }
  mgr := vm.NewEVMManager(s, int64(chainid), vm.Config{}, chaincfg)
  tm.Register("eth", api.NewETHAPI(s, mgr, int64(chainid)))
  tm.Register("web3", &api.Web3API{})
  tm.Register("net", &api.NetAPI{chaincfg.NetworkID})
  if err := tm.Run(); err != nil {
    log.Error("Critical Error. Shutting down.", "error", err)
    db.Close()
    os.Exit(1)
  }
}
