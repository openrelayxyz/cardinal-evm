package api

import (
	"context"
	"math/big"
	"testing"

	// log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	// "github.com/openrelayxyz/cardinal-types/hexutil"

	// "github.com/openrelayxyz/cardinal-storage"
	ctypes "github.com/openrelayxyz/cardinal-types"
)

func TestEVMApi (t *testing.T){
	chainID := int64(1)
	sdb := state.NewMemStateDB(chainID, 128)

	mgr := vm.NewEVMManager(sdb.Storage, chainID, vm.Config{}, &params.ChainConfig{})
	e := NewETHAPI(sdb.Storage, mgr, chainID, func(*types.Header) uint64 {return 30000000})
	t.Run("BlockNumber", func(t *testing.T){
		sdb.Storage.AddBlock(
			ctypes.HexToHash("0x01"), 
			ctypes.Hash{},           
			1,                       
			big.NewInt(1),     
			nil, nil, []byte("0"),
		)

		test, err := e.BlockNumber(rpc.NewContext(context.Background()))
		if err != nil {
			t.Fatal(err.Error())
		}
		if test != 1 {
			t.Fatalf("Blocknumber result not accurate")
		}
	})

	t.Run("GetBalance", func(t *testing.T){
		addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000000")
		bal := big.NewInt(42)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: ctypes.Hash{},
			Difficulty: big.NewInt(2), 
		}

		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader) 

		sdb.Storage.AddBlock(blockHash,
			 header.ParentHash, 
			 header.Number.Uint64(), 
			 header.Difficulty,
			 nil, nil, 
			 rawHeader,
		)
		err := mgr.View(blockHash, func (st state.StateDB){
			st.CreateAccount(addr)
			st.SetBalance(addr, bal)
			st.Finalise()
		})

		if err != nil {
			t.Fatalf("View failed: %v", err)
		}

		 test, err := e.GetBalance(rpc.NewContext(context.Background()), addr, vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err!=nil{
			t.Fatal(err.Error())
		}

		if bal.Cmp((*big.Int)(test)) != 0 {
			t.Fatalf("error in getBalance, expected å%s, got %s", bal.String(), test.String())
		}
	})

	t.Run("GetCode", func(t *testing.T){
	})

	t.Run("GetStorageAt", func(t *testing.T){

	})

	t.Run("EstimateGas", func(t *testing.T){

	})

	t.Run("Call", func(t *testing.T){
		
	})
	
}
