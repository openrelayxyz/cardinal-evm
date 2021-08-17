package vm

import (
  "math/big"
  // "github.com/openrelayxyz/cardinal-storage/db"
  "github.com/openrelayxyz/cardinal-storage"
  "github.com/openrelayxyz/cardinal-evm/common"
  "github.com/openrelayxyz/cardinal-evm/params"
  "github.com/openrelayxyz/cardinal-evm/rlp"
  "github.com/openrelayxyz/cardinal-evm/schema"
  "github.com/openrelayxyz/cardinal-evm/state"
  "github.com/openrelayxyz/cardinal-evm/types"
  // log "github.com/inconshreveable/log15"
  ctypes "github.com/openrelayxyz/cardinal-types"
)

type EVMManager struct{
  sdbm     *state.StatedbManager
  vmcfg    Config
  chaincfg *params.ChainConfig
}

func (mgr *EVMManager) ViewEVM(h ctypes.Hash, from common.Address, gasPrice *big.Int, fn func(storage.Transaction, state.StateDB, *EVM) error) error {
  return mgr.sdbm.View(h, func (tx storage.Transaction, statedb state.StateDB) error {
    header := &types.Header{}
    if err := tx.ZeroCopyGet(schema.BlockHeader(mgr.sdbm.Chainid, h.Bytes()), func(data []byte) error {
      return rlp.DecodeBytes(data, &header)
    }); err != nil { return err }

    blockCtx := BlockContext{
      GetHash:     tx.NumberToHash,
      Coinbase:    header.Coinbase,
      GasLimit:    header.GasLimit,
      BlockNumber: header.Number,
      Time:        new(big.Int).SetUint64(header.Time),
      Difficulty:  header.Difficulty,
      BaseFee:     header.BaseFee,
    }

    evm := NewEVM(blockCtx, TxContext{from, gasPrice}, statedb, mgr.chaincfg, mgr.vmcfg)

    return fn(
      tx,
      statedb,
      evm,
    )
  })
}
