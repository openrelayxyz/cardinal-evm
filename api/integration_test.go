package api

import (
	// "bytes"
	"context"
	"crypto/ecdsa"
	// "errors"
	"fmt"
	"math/big"
	// "reflect"
	// "regexp"
	"slices"
	// "strings"

	"testing"

	log "github.com/inconshreveable/log15"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	// "github.com/holiman/uint256"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	// "github.com/stretchr/testify/require"

	"github.com/openrelayxyz/cardinal-storage"
)
type account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
} 

// estimateGasErrorRatio is the amount of overestimation eth_estimateGas is
// allowed to produce in order to speed up calculations.
const estimateGasErrorRatio = 0.015

var gasLimit  = uint64(30000000)

func newAccounts(n int) (accounts []account) {
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)
		accounts = append(accounts, account{key: key, addr: addr})
	}
	slices.SortFunc(accounts, func(a, b account) int { return a.addr.Cmp(b.addr) })
	return accounts
}

// i'm using a mock emitter to mock the TransactionEmitter in SendRawTransaction's implementation
// this stored the signed transaction in the seen field 
type mockEmitter struct {
	seen *types.Transaction
}
func (m *mockEmitter) Emit(tx *types.Transaction) error {
	m.seen = tx;
	return nil
}

type TransactionEmitterFunc func(*types.Transaction) error

func (f TransactionEmitterFunc) Emit(tx *types.Transaction) error {
	return f(tx)
}

func newRPCBalance(balance *big.Int) **hexutil.Big {
	rpcBalance := (*hexutil.Big)(balance)
	return &rpcBalance
}

func hex2Bytes(str string) *hexutil.Bytes {
	rpcBytes := hexutil.Bytes(common.FromHex(str))
	return &rpcBytes
}

func fund (bal int64) []byte {
	enc, _ := rlp.EncodeToBytes(state.Account{Balance: big.NewInt(bal)})
	return enc
}

func fundWithCode(balance int64, code []byte) []byte {
    account := state.Account{
        Balance: big.NewInt(balance),
        CodeHash: crypto.Keccak256Hash(code).Bytes(),
    }
    enc, _ := rlp.EncodeToBytes(account)
    return enc
}

func fundWithNonce(bal int64, nonce uint64) []byte {
	enc, _ := rlp.EncodeToBytes(state.Account{
		Balance: big.NewInt(bal),
		Nonce:   nonce,
	})
	return enc
}

func addGenesis(sdb *state.StatedbManager, chainID int64) ctypes.Hash {
    genesisHeader := &types.Header{Number: big.NewInt(0), GasLimit: gasLimit}
    rawGenesisHeader, _ := rlp.EncodeToBytes(genesisHeader)
    genesisHash := crypto.Keccak256Hash(rawGenesisHeader)
    
    sdb.Storage.AddBlock(genesisHash, 
		ctypes.Hash{},
		 0, 
		 big.NewInt(0), 
        []storage.KeyValue{{Key: schema.BlockHeader(chainID, genesisHash.Bytes()), Value: rawGenesisHeader}}, 
        nil, []byte("0"))
    
    return genesisHash
}

func setupEVMTest(t *testing.T) (*vm.EVMManager, *params.ChainConfig, *state.StatedbManager, int64) {
	t.Helper()
	chainID := int64(1)
    sdb := state.NewMemStateDB(chainID, 128)
    
    cfg := *params.AllEthashProtocolChanges
    cfg.ChainID = big.NewInt(chainID)
    cfg.ShanghaiBlock = big.NewInt(0)
    mgr := vm.NewEVMManager(sdb.Storage, chainID, vm.Config{}, &cfg)
    
    return mgr, &cfg, sdb, chainID
}


func TestEVMApi (t *testing.T){
	// web3Api :=&Web3API{}

	t.Run("BlockNumber", func (t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		// genesisHash := addGenesis(sdb, chainId)

		// block1Hash := ctypes.HexToHash("0x01")
		// sdb.Storage.AddBlock(
		// 	block1Hash,
		// 	genesisHash, 
		// 	1, 
		// 	big.NewInt(1), nil, nil, 
		// 	[]byte("1"),
		// )
	
		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})

		// (ctx *rpc.CallContext, opts simOpts, blockNrOrHash *vm.BlockNumberOrHash)
		
		// [{"blockStateCalls":[{"blockOverrides":{"baseFeePerGas":"0x9","gasLimit": "0x1c9c380"},
		// "stateOverrides":{"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045":{"balance":"0x4a817c420"}},
		// "calls":[{"from":"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045","to":"0x014d023e954bAae7F21E56ed8a5d81b12902684D",
		// "gas":"0x5208",
		// "maxFeePerGas":"0xf",
		// "value":"0x1"}]}],
		// "validation":true,
		// "traceTransfers":true},
		// "latest"]

		// arg := simOpts{
		// 	BlockStateCalls: []simBlock
		// }




		res, err := e.SimulateV1(rpc.NewContext(context.Background()), opts, "latest")
		if err != nil {
			t.Fatal(err.Error())
		}
		log.Error("this is the res", "res", res)
		if res != 0 {
			t.Errorf(fmt.Sprintf("Blocknumber result not accurate, result, %v", res))
		}
	})

}