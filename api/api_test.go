package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"math/big"
	"slices"
	"testing"

	// log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"

	"github.com/openrelayxyz/cardinal-storage"
)
type account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

func newAccounts(n int) (accounts []account) {
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)
		accounts = append(accounts, account{key: key, addr: addr})
	}
	slices.SortFunc(accounts, func(a, b account) int { return a.addr.Cmp(b.addr) })
	return accounts
}


func TestEVMApi (t *testing.T){
	chainID := int64(1)
	sdb := state.NewMemStateDB(chainID, 128)

	genesisHeader := &types.Header{
		Number: big.NewInt(0),
		 GasLimit: 30000000,
	}
	rawGenesisHeader, _ := rlp.EncodeToBytes(genesisHeader)
	genesisHash := ctypes.Hash(crypto.Keccak256Hash(rawGenesisHeader))

	sdb.Storage.AddBlock(
		genesisHash, 
		ctypes.Hash{}, 
		0, 
		big.NewInt(0), 
		[]storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, genesisHash.Bytes()), Value: rawGenesisHeader },
		}, nil, 
		[]byte("0"),
)

	mgr := vm.NewEVMManager(sdb.Storage, chainID, vm.Config{}, params.AllEthashProtocolChanges)
	e := NewETHAPI(sdb.Storage, mgr, chainID, func(*types.Header) uint64 {return 30000000})

	t.Run("BlockNumber", func(t *testing.T){
		block1Hash := ctypes.HexToHash("0x01")
		sdb.Storage.AddBlock(
			block1Hash,
			genesisHash, 
			1, 
			big.NewInt(1), nil, nil, 
			[]byte("1"),
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
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2), 
		}

		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader) 

		account := state.Account{Balance: bal}
		encodedAccount, _ := rlp.EncodeToBytes(account)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainID, addr.Bytes()), Value: encodedAccount},
		}
		sdb.Storage.AddBlock(
			 blockHash,
			 header.ParentHash, 
			 header.Number.Uint64(), 
			 header.Difficulty,
			 updates,
			 nil, []byte("1"),
		)

		 test, err := e.GetBalance(rpc.NewContext(context.Background()), addr, vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err!=nil{
			t.Fatal(err.Error())
		}
		if bal.Cmp((*big.Int)(test)) != 0 {
			t.Fatalf("error in getBalance, expected å%s, got %s", bal.String(), test.String())
		}
	})

	t.Run("Call", func(t *testing.T){
		accounts := newAccounts(3)
		from := accounts[0].addr
		to := accounts[1].addr


		contract := hexutil.MustDecode("0x6080604052348015600f57600080fd5b506004361060285760003560e01c80638381f58a14602d575b600080fd5b60336049565b6040518082815260200191505060405180910390f35b6000548156fea2646970667358221220eab35ffa6ab2adfe380772a48b8ba78e82a1b820a18fcb6f59aa4efb20a5f60064736f6c63430007040033")
		contractHash := crypto.Keccak256Hash(contract)

		fromAccount := state.Account{Balance: big.NewInt(1e18)}
		toAccount := state.Account{CodeHash: contractHash.Bytes()}
		encodedFrom, _ := rlp.EncodeToBytes(fromAccount)
		encodedTo, _ := rlp.EncodeToBytes(toAccount)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(1),
		}
		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		storageSlot := ctypes.Hash{} 
		hashedSlot := crypto.Keccak256Hash(storageSlot.Bytes())
		storageValue := ctypes.BigToHash(big.NewInt(123)) 

		rlpEncodedValue, _ := rlp.EncodeToBytes(storageValue.Bytes())

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainID, from.Bytes()), Value: encodedFrom},
			{Key: schema.AccountData(chainID, to.Bytes()), Value: encodedTo},
			{Key: schema.AccountCode(chainID, contractHash.Bytes()), Value: contract},
			{Key: schema.AccountStorage(chainID, to.Bytes(), hashedSlot.Bytes()), Value: rlpEncodedValue},
		}

		sdb.Storage.AddBlock(blockHash, 
			header.ParentHash, 
			header.Number.Uint64(), 
			header.Difficulty, updates, nil, 
			[]byte("1"),
		)

		data := hexutil.Bytes(hexutil.MustDecode("0x8381f58a"))
		gas := hexutil.Uint64(100000)
		args := TransactionArgs{
			From: &from,
			To:   &to,
			Gas:  &gas,
			Data: &data, 
		}

		res, err := e.Call(rpc.NewContext(context.Background()), args, vm.BlockNumberOrHash{BlockHash: &blockHash}, nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		want := hexutil.Bytes(ctypes.BigToHash(big.NewInt(123)).Bytes())
		resBytes, ok := res.(hexutil.Bytes)
		if !ok {
			t.Fatalf("unexpected result type: %T", res)
		}
		if !bytes.Equal(resBytes, want) {
			t.Fatalf("unexpected result: got %s, want %s", resBytes.String(), want.String())
		}
	})

	t.Run("GetCode", func(t *testing.T){
	})

	t.Run("GetStorageAt", func(t *testing.T){

	})

	t.Run("EstimateGas", func(t *testing.T){

	})
}
