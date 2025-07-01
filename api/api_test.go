package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"math/big"
	"regexp"
	"slices"
	"strings"
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
	"github.com/stretchr/testify/require"

	"github.com/openrelayxyz/cardinal-storage"
)
type account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}
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

func TestEVMApi (t *testing.T){
	chainID := int64(1)
	sdb := state.NewMemStateDB(chainID, 128)

	genesisHeader := &types.Header{
		Number: big.NewInt(0),
		 GasLimit: 30000000,
	}
	rawGenesisHeader, _ := rlp.EncodeToBytes(genesisHeader)
	genesisHash := ctypes.Hash(crypto.Keccak256Hash(rawGenesisHeader))

	sdb.Storage.AddBlock(genesisHash, 
		ctypes.Hash{}, 0, 
		big.NewInt(0), 
		[]storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, genesisHash.Bytes()), Value: rawGenesisHeader },
		}, nil, 

		[]byte("0"),
    )
	cfg := *params.AllEthashProtocolChanges
	cfg.ShanghaiBlock = big.NewInt(0)
	mgr := vm.NewEVMManager(sdb.Storage, chainID, vm.Config{}, &cfg)
	e := NewETHAPI(sdb.Storage, mgr, chainID, func(*types.Header) uint64 {return gasLimit})
	web3Api :=&Web3API{}
	debugApi := NewDebugAPI(sdb.Storage, mgr, chainID)

	t.Run("BlockNumber", func(t *testing.T){
		block1Hash := ctypes.HexToHash("0x01")
		sdb.Storage.AddBlock(
			block1Hash,
			genesisHash, 
			1, 
			big.NewInt(1), nil, nil, 
			[]byte("1"),
		)

		res, err := e.BlockNumber(rpc.NewContext(context.Background()))
		if err != nil {
			t.Fatal(err.Error())
		}
		if res != 1 {
			t.Fatalf("Blocknumber result not accurate")
		}
	})

	t.Run("GetBalance", func(t *testing.T){
		addr := newAccounts(1)[0].addr
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

		result, err := e.GetBalance(rpc.NewContext(context.Background()), addr, vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err!=nil{
			t.Fatal(err.Error())
		}
		if bal.Cmp((*big.Int)(result)) != 0 {
			t.Fatalf("error in getBalance, expected %s, got %s", bal.String(), result.String())
		}
	})

	t.Run("Call", func(t *testing.T){
		accounts := newAccounts(3)
		from, to := accounts[0].addr, accounts[1].addr

		contract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80633fa4f24514602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212201ac5f57785e601674d20159494ae12fb2cc62ec0a9152929e20dfa835723007b64736f6c634300081e0033"))
		contractHash := crypto.Keccak256Hash(contract)

		fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
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

		// Contract storage Key and Value
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

		data := hexutil.Bytes(ctypes.Hex2Bytes("3fa4f245")) // function value()
		gas := hexutil.Uint64(100000)
		args := TransactionArgs{
			From: &from,
			To:   &to,
			Gas:  &gas,
			Data: &data, 
		}

		test, err := e.Call(rpc.NewContext(context.Background()), args, vm.BlockNumberOrHash{BlockHash: &blockHash}, nil)
		if err != nil {
			t.Fatal(err.Error())
		}

		actual := ctypes.BigToHash(big.NewInt(123)).Bytes()
		testBytes, ok := test.(hexutil.Bytes)
		
		if !ok {
			t.Fatalf("unexpected result type: %T", test)
		}
		if !bytes.Equal(testBytes, actual) {
			t.Fatalf("error in eth_call, expected %s, got %s", string(actual),testBytes.String())
		}
	})

	t.Run("GetCode", func(t *testing.T){
		accounts := newAccounts(2)
		address, contractAddr := accounts[0].addr,accounts[1].addr

		contract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80633fa4f24514602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212201ac5f57785e601674d20159494ae12fb2cc62ec0a9152929e20dfa835723007b64736f6c634300081e0033"))
		contractHash := crypto.Keccak256Hash(contract)

		addressAcct := state.Account{Balance: big.NewInt(params.Ether)}
		contractAcct := state.Account{CodeHash: contractHash.Bytes()}

		encodedAddress,_ := rlp.EncodeToBytes(addressAcct)
		encodedContract,_ := rlp.EncodeToBytes(contractAcct)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(1),
		}
		rawHeader,_ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader },
			{ Key: schema.AccountData(chainID, address.Bytes()),Value: encodedAddress },
			{ Key: schema.AccountData(chainID, contractAddr.Bytes()), Value: encodedContract },
			{ Key: schema.AccountCode(chainID, contractHash.Bytes()), Value: contract},
		}
		
		sdb.Storage.AddBlock(
			blockHash, 
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates,
			nil, 
			[]byte("1"),
		)

		result, err := e.GetCode(rpc.NewContext(context.Background()), contractAddr, vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err != nil {
			t.Fatal(err.Error())
		}
		resBytes, ok := result.(hexutil.Bytes)
		if !ok {
			t.Fatalf("unexpected result type %T", result )
		}

		if !bytes.Equal(resBytes, contract){
			t.Fatalf("error in getCode, expected %s, got %s", string(contract), resBytes.String())
		}
	})

	t.Run("CreateAccessList", func(t *testing.T){
		accounts := newAccounts(2)
		from, to := accounts[0].addr, accounts[1].addr

		// SPDX-License-Identifier: MIT
		// pragma solidity ^0.8.0;
		//
		// contract SimpleStorage {
		//     uint256 private value;
		//
		//     function retrieve() public view returns (uint256) {
		//         return value;
		//     }
		// }

		contract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80632e64cec114602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212202ba4aea8a5151d55cb5bd6817fe8aa9ea5f8c07496296377b5cd34635676678264736f6c634300081e0033"))
		codeHash := crypto.Keccak256Hash(contract)
		fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
		toAccount := state.Account{CodeHash: codeHash.Bytes()}

		encodedFrom,_ := rlp.EncodeToBytes(fromAccount)
		encodedTo,_ := rlp.EncodeToBytes(toAccount)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(1),
		}

		rawHeader,_ :=rlp.EncodeToBytes(header)
		blockhash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, blockhash.Bytes()), Value: rawHeader},
			{ Key: schema.AccountData(chainID, from.Bytes()), Value: encodedFrom},
			{ Key: schema.AccountData(chainID, to.Bytes()), Value: encodedTo},
			{ Key: schema.AccountCode(chainID, codeHash.Bytes()), Value: contract},
		}

		sdb.Storage.AddBlock(blockhash, 
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates, nil,
			[]byte("1"),
		)

		data := hexutil.Bytes(ctypes.Hex2Bytes("2e64cec1"))
		gas := hexutil.Uint64(100000)
		args := TransactionArgs{
			From:  &from,
			To:    &to,
			Data:  &data,
			Gas:   &gas,
			Value: new(hexutil.Big),
		}

		result, err := e.CreateAccessList(rpc.NewContext(context.Background()), args, &vm.BlockNumberOrHash{BlockHash: &blockhash})
		if err != nil {
			t.Fatalf(err.Error())
		}
		if result.Error != "" {
			t.Fatalf("CreateAccessList returned a VM error: %v", result.Error)
		}
		if result.Accesslist == nil {
			t.Fatal("expected an access list, but got nil")
		}

		expected := &types.AccessList{
			{
				Address: to,
				StorageKeys: []ctypes.Hash{{}},
			},
		}

		require.Equal(t, expected, result.Accesslist)
	})

	t.Run("GetStorageAt", func(t *testing.T){
		creator := newAccounts(1)[0].addr
		contract := crypto.CreateAddress(creator, 0)

		storageSlot := ctypes.HexToHash("0x01") 
		hashedSlot := crypto.Keccak256Hash(storageSlot.Bytes())
		storageValue := ctypes.HexToHash("0xdeadbeef") 
		rlpEncodedValue, _ := rlp.EncodeToBytes(storageValue.Bytes())

		header := &types.Header{
			Number: big.NewInt(2),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2),
		}
		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountStorage(chainID, contract.Bytes(), hashedSlot.Bytes()), Value: rlpEncodedValue},
		}

		sdb.Storage.AddBlock(blockHash,
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates, nil,
			[]byte("2"),
		)

		actual := storageValue.Bytes()
		test, err := e.GetStorageAt(rpc.NewContext(context.Background()), contract, storageSlot.Hex(), vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err != nil {
			t.Fatal(err.Error())
		}
		testBytes, ok := test.(hexutil.Bytes)
		if !ok {
			t.Fatalf("unexpected result type: %T", test)
		}
		if !bytes.Equal(testBytes, actual) {
			t.Fatalf("error in eth_call, expected %s, got %s", string(actual),testBytes.String())
		}

	})

	t.Run("EstimateGas", func(t *testing.T){

	})
	
	t.Run("Debug_TraceStructLog", func(t *testing.T){
		accounts := newAccounts(2)
		from, to := accounts[0].addr, accounts[1].addr 

		contract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106030575f3560e01c80633fa4f2451460345780635601eaea14604d575b5f5ffd5b603b5f5481565b60405190815260200160405180910390f35b605c6058366004606e565b605e565b005b5f60678284608d565b5f55505050565b5f5f60408385031215607e575f5ffd5b50508035926020909101359150565b8082018082111560ab57634e487b7160e01b5f52601160045260245ffd5b9291505056fea2646970667358221220cb70395670a2ececa377ddb2a0d7d0d24b58421678f45e6a00fa6416d23cee7664736f6c634300081e0033"))
		codeHash := crypto.Keccak256Hash(contract)
		fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
		toAccount := state.Account{CodeHash: codeHash.Bytes()}

		encodedFrom,_ := rlp.EncodeToBytes(fromAccount)
		encodedTo,_ := rlp.EncodeToBytes(toAccount)

		header := &types.Header{
			Number: big.NewInt(4),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(1),
		}

		rawHeader,_ :=rlp.EncodeToBytes(header)
		blockhash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, blockhash.Bytes()), Value: rawHeader},
			{ Key: schema.AccountData(chainID, from.Bytes()), Value: encodedFrom},
			{ Key: schema.AccountData(chainID, to.Bytes()), Value: encodedTo},
			{ Key: schema.AccountCode(chainID, codeHash.Bytes()), Value: contract},
		}

		sdb.Storage.AddBlock(
			blockhash, 
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates, nil,
			[]byte("1"),
		)

		data := hexutil.Bytes(append(ctypes.Hex2Bytes("5601eaea"), ctypes.LeftPadBytes(big.NewInt(10).Bytes(), 32)...))
		data = append(data, ctypes.LeftPadBytes(big.NewInt(20).Bytes(), 32)...)
		gas := hexutil.Uint64(150000)
 
		args := TransactionArgs{
			From: &from,
			To: &to,
			Gas: &gas,
			Data: &data,
		}

		logs, err := debugApi.TraceStructLog(rpc.NewContext(context.Background()), args, &vm.BlockNumberOrHash{BlockHash: &blockhash})
		if err!=nil {
			t.Fatalf(err.Error())
		}
		if len(logs) == 0 {
			t.Fatal("Empty Trace")
		}

		var addOk, storeOk bool
		for _, l := range logs {
			switch l.Op {
			case vm.ADD:
				s := l.Stack
				if len(s) >= 2  && ((s[len(s)-1].Uint64() == 10 && s[len(s)-2].Uint64() == 20)  || (s[len(s)-1].Uint64() == 20 && s[len(s)-2].Uint64() == 10) ){
					addOk = true
				}
			case vm.SSTORE:
				s:= l.Stack
				if len(s) >= 2 && s[len(s)-1].IsZero() &&  s[len(s)-2].Uint64() == 30 {
					storeOk = true
				}
   			}
		}
		if !addOk{
			t.Fatalf("error in debug_tracestructlog, Add(10,20) not found or stack wrong")
		}
		if !storeOk {
			t.Fatalf("error in debug_tracestructlog, SSTORE(slot 0, 30) not found or stack wrong")
		}
		if logs[len(logs)-1].Err != nil {
			t.Fatalf("error in debug_tracestructlog, trace ended with error: %v", logs[len(logs)-1].Err)
		}
	})

	t.Run("web3_clientVersion", func(t *testing.T) {
		version := web3Api.ClientVersion()
		if version == "" {
			t.Fatal("ClientVersion returned empty string")
		}
		if !strings.HasPrefix(version, "CardinalEVM/") {
			t.Fatalf("unexpected prefix: %s", version)
		}
		matched, _ := regexp.MatchString(`^CardinalEVM/.+/.+/go\d+\.\d+(\.\d+)?$`, version)
		if !matched {
			t.Fatalf("invalid version format: %s", version)
		}
	})
}
