package vm

import (
  "context"
  "testing"
  "math/big"
  "github.com/openrelayxyz/cardinal-storage"
  "github.com/openrelayxyz/cardinal-types"
  "github.com/openrelayxyz/cardinal-evm/common"
  "github.com/openrelayxyz/cardinal-evm/state"
  "github.com/openrelayxyz/cardinal-evm/params"
  "github.com/openrelayxyz/cardinal-evm/rlp"
  etypes "github.com/openrelayxyz/cardinal-evm/types"
)

func loadedManager(records []storage.KeyValue) *EVMManager {
  sdb := state.NewMemStateDB(1, 128)
  sdb.Storage.AddBlock(
    types.HexToHash("a"),
    types.Hash{},
    1,
    big.NewInt(1),
    records,
    [][]byte{},
    []byte("0"),
  )
  return &EVMManager{
    sdbm: sdb,
    vmcfg: Config{},
    chaincfg: &params.ChainConfig{},
  }
}
func defaultManager() *EVMManager {
  return loadedManager([]storage.KeyValue{
    {Key: []byte("/a/Data"), Value: []byte("Something")},
    {Key: []byte("/a/Data2"), Value: []byte("Something Else")},
    {Key: []byte("a"), Value: []byte("1")},
    {Key: []byte("b"), Value: []byte("2")},
  })
}
func managerWithHeader() *EVMManager {
  x, _ := rlp.EncodeToBytes(&etypes.Header{
    Difficulty: new(big.Int).SetInt64(1),
    Number: new(big.Int).SetInt64(1),
    GasLimit: 15000000,
    GasUsed: 0,
    Time: 1629319372000,
    BaseFee: big.NewInt(1000000000),
  })
  return loadedManager([]storage.KeyValue{
    {
      Key: []byte("c/1/b/000000000000000000000000000000000000000000000000000000000000000a/h"),
      Value: x,
    },
  })
}


func TestViewHash(t *testing.T) {
  mgr := defaultManager()
  err := mgr.View(func(h types.Hash){
    if h != types.HexToHash("a") { t.Errorf("Unexpected hash value: %#x", h.Bytes())}
  })
  if err != nil { t.Errorf(err.Error())}
}
func TestViewHashBn(t *testing.T) {
  mgr := defaultManager()
  err := mgr.View(BlockNumberOrHashWithNumber(1), func(h types.Hash){
    if h != types.HexToHash("a") { t.Errorf("Unexpected hash value: %#x", h.Bytes())}
  })
  if err != nil { t.Errorf(err.Error())}
}

func TestViewUnexpectedInput(t *testing.T) {
  mgr := defaultManager()
  err := mgr.View("thing", func(){})
  if err == nil { t.Errorf("Expected unknown parameter error")}
}

func TestViewUnexpectedCallbackInput(t *testing.T) {
  mgr := defaultManager()
  err := mgr.View(func(string){})
  if err == nil { t.Errorf("Expected unknown parameter error")}
}

func TestViewBlockNumber(t *testing.T) {
  mgr := managerWithHeader()
  err := mgr.View(func(num uint64){
    if num != 1 { t.Errorf("Unexpected block number")}
  })
  if err != nil { t.Errorf(err.Error())}
}

func TestViewTx(t *testing.T) {
  mgr := managerWithHeader()
  err := mgr.View(func(hash types.Hash, tx storage.Transaction){
    if tx.HashToNumber(hash) != 1 { t.Errorf("Unexpected hash number")}
  })
  if err != nil { t.Errorf(err.Error())}
}

func TestViewHeader(t *testing.T) {
  mgr := managerWithHeader()
  err := mgr.View(types.HexToHash("a"), func(h types.Hash, header *etypes.Header){
    if header.GasLimit != 15000000 { t.Errorf("Unexpected gas limit") }
    if header.BaseFee.Cmp(big.NewInt(1000000000)) != 0 { t.Errorf("Unexpected base fee")}
  })
  if err != nil { t.Errorf(err.Error())}
}

func TestStatedb(t *testing.T) {
  mgr := managerWithHeader()
  err := mgr.View(context.Background(), func(state state.StateDB){
    if state.GetNonce(common.Address{}) != 0 { t.Errorf("Unexpected nonce") }
  })
  if err != nil { t.Errorf(err.Error())}
}

func TestViewEVM(t *testing.T) {
  mgr := managerWithHeader()
  err := mgr.View(context.Background(), func(evm *EVM){
    evm.Cancel()
    if !evm.Cancelled() { t.Errorf("EVM should have been cancelled" )}
  })
  if err != nil { t.Errorf(err.Error())}
}


// case headerType:
//   needsTx = true
//   needsHeader = true
//   headerPos = i
// case statedbType:
//   needsTx = true
//   needsHeader = true
//   needsState = true
//   statePos = i
// case evmType:
//   needsTx = true
//   needsHeader = true
//   needsState = true
//   evmPos = i
