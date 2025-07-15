package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"testing"

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

func TestEVMApi (t *testing.T){
	chainID := int64(1)
	sdb := state.NewMemStateDB(chainID, 128)

	genesisHeader := &types.Header{
		Number: big.NewInt(0),
		GasLimit: gasLimit,
	}
	rawGenesisHeader, _ := rlp.EncodeToBytes(genesisHeader)
	genesisHash := crypto.Keccak256Hash(rawGenesisHeader)

	sdb.Storage.AddBlock(genesisHash, 
		ctypes.Hash{}, 0, 
		big.NewInt(0), 
		[]storage.KeyValue{
			{ Key: schema.BlockHeader(chainID, genesisHash.Bytes()), Value: rawGenesisHeader },
		}, nil, 

		[]byte("0"),
    )
	cfg := *params.AllEthashProtocolChanges
	cfg.ChainID = big.NewInt(chainID)
	cfg.ShanghaiBlock = big.NewInt(0)
	mgr := vm.NewEVMManager(sdb.Storage, chainID, vm.Config{}, &cfg)

	e := NewETHAPI(sdb.Storage, mgr, chainID, func(*types.Header) uint64 {return gasLimit})
	web3Api :=&Web3API{}
	debugApi := NewDebugAPI(sdb.Storage, mgr, chainID)
	// ethercattleApi := NewEtherCattleBlockChainAPI(mgr, func(*types.Header) uint64 {return gasLimit})

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

		var testSuite = [] struct {
			name string
			balance *big.Int
			want *big.Int
			expectErr error
		}{
			{
				name: "non-zero",
				balance: bal,
				want: bal,
				expectErr: nil,
			},
			{
				name: "zero-balance", 
				balance: big.NewInt(0), 
				want: big.NewInt(0), 
				expectErr: nil,
			},
			{
				name: "missing-account",
				balance: nil,
				want: big.NewInt(0),
				expectErr: nil,
			},
			{
				name: "bad-block",
				balance: bal,
				want: nil,
				expectErr: storage.ErrLayerNotFound,
			},
		}

		for i, tc := range testSuite {
			var blockHash ctypes.Hash

			if tc.expectErr != nil{
				fa := newAccounts(1)[0].addr
				blockHash  = crypto.Keccak256Hash(fa.Bytes())
			} else {
				header := &types.Header{
					Number: big.NewInt(int64(i + 1)),
					ParentHash: genesisHash,
					Difficulty: big.NewInt(int64(i + 1)), 
					GasLimit: gasLimit,
				}

				rawHeader, _ := rlp.EncodeToBytes(header)
				blockHash = crypto.Keccak256Hash(rawHeader) 

				updates := []storage.KeyValue{
					{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
				}

				if tc.balance != nil {
					encodedAccount, _ := rlp.EncodeToBytes(state.Account{Balance: tc.balance})
					updates = append(updates, storage.KeyValue{
						Key: schema.AccountData(chainID, addr.Bytes()), Value: encodedAccount,
					})
				}

				sdb.Storage.AddBlock(
					blockHash,
					header.ParentHash, 
					header.Number.Uint64(), 
					header.Difficulty,
					updates,
					nil, []byte("1"),
			   )
			}
			result, err := e.GetBalance(rpc.NewContext(context.Background()), addr, vm.BlockNumberOrHash{BlockHash: &blockHash})
			   if tc.expectErr != nil{
				if err == nil {
					t.Errorf("test %d: want error %v, have nothing", i, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr) {
					if err.Error() != tc.expectErr.Error() {
						t.Fatalf("%v: expected error %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			   }
				if err!=nil{
					t.Fatal(err.Error())
				}
				if(*big.Int)(result).Cmp(tc.want) != 0 {
					t.Fatalf("error in getBalance, %s: balance mismatch. expected %s, got %s", tc.name, tc.want.String(), (*big.Int)(result).String())
				}
		}
	})

	t.Run("GetCode", func(t *testing.T){
		accounts := newAccounts(1)
		address := accounts[0].addr

		// ConfigurableValue
		contractA := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80633fa4f24514602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212201ac5f57785e601674d20159494ae12fb2cc62ec0a9152929e20dfa835723007b64736f6c634300081e0033"))

		// DoubleStore
		contractB := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80636d4ce63c14602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea264697066735822122021f2db8e56187d1ecda9ce08724e0a64379f499aa7551783058fe1cc8812512364736f6c634300081e0033"))
		
		var testSuite = []struct {
			name string
			code []byte
			want []byte
			expectErr error
		}{
			{
				name: "contractA",
				code: contractA,
				want: contractA,
			},
			{
				name: "contractB",
				code: contractB,
				want: contractB,
			},
			{
				name: "empty-code",
				code: []byte{},
				want: []byte{},
			},
			{
				name: "bad-block",
				code: contractA,
				want: nil,
				expectErr: storage.ErrLayerNotFound,
			},
			{
				name: "missing-account",
				code: nil,
				want: []byte{},
			},
		}

		for i, tc := range testSuite{
			var blockHash ctypes.Hash

			if tc.expectErr != nil{
				blockHash = crypto.Keccak256Hash(newAccounts(1)[0].addr.Bytes())
			} else{
				header := &types.Header{
					Number: big.NewInt(int64(i + 1)),
					ParentHash: genesisHash,
					Difficulty: big.NewInt(int64(i + 1)), 
					GasLimit: gasLimit,
				}
				rawHeader,_ := rlp.EncodeToBytes(header)
				blockHash = crypto.Keccak256Hash(rawHeader)

				updates := []storage.KeyValue{
					{ Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader },
				}

				if tc.code != nil {
					var contractHash ctypes.Hash 
					
					if len(tc.code) > 0 {
						contractHash = crypto.Keccak256Hash(tc.code)
						updates = append(updates, storage.KeyValue{
							Key: schema.AccountCode(chainID, contractHash.Bytes()),
							Value: tc.code,
						})
					} else{
						contractHash = crypto.Keccak256Hash(nil)
					}

					contractAcct := state.Account{CodeHash: contractHash.Bytes()}
					encodedContract, _ := rlp.EncodeToBytes(contractAcct)
					updates = append(updates, storage.KeyValue{
						Key: schema.AccountData(chainID, address.Bytes()),
						Value: encodedContract,
					})
				} 

				sdb.Storage.AddBlock(blockHash, 
					header.ParentHash,
					header.Number.Uint64(),
					header.Difficulty,
					updates, nil, 
					[]byte("1"),
				)
			}
			result, err := e.GetCode(rpc.NewContext(context.Background()), address, vm.BlockNumberOrHash{BlockHash: &blockHash})
			if tc.expectErr != nil{
				if err == nil {
					t.Errorf("test %d: want error %v, have nothing", i, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr){
					if err.Error() != tc.expectErr.Error() {
						t.Fatalf("%s: expected %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Fatal(err.Error())
			}
			resBytes, ok := result.(hexutil.Bytes)
			if !ok {
				t.Fatalf("unexpected result type %T", result )
			}

			if !bytes.Equal(resBytes, tc.want){
				t.Fatalf("error in getCode, %v : expected %v, actual %v",  tc.name, tc.want, resBytes.String())
			}
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

		storageValue := ctypes.HexToHash("0xdeadbeef") 
		rlpEncodedValue, _ := rlp.EncodeToBytes(storageValue.Bytes())
		zeroHash := ctypes.Hash{} 
		rlpZero, _ := rlp.EncodeToBytes(zeroHash.Bytes())

		var testSuite = [] struct {
			name string
			target common.Address
			slotHex string
			writeVal []byte
			want []byte
			expectErr error
		}{
			{
				name: "value-present",
				target: contract,
				slotHex: "0x01",
				writeVal: rlpEncodedValue,
				want:storageValue.Bytes(),
			},
			{
				name: "zero-value",
				target: contract,
				slotHex: "0x02",
				writeVal: rlpZero,
				want: zeroHash.Bytes(),
			},
			{
				name: "slot-missing",
				target: contract,
				slotHex: "0x03",
				writeVal: nil,
				want: zeroHash.Bytes(),
			},
			{
				name: "missing-account",
				target: crypto.CreateAddress(creator, 1),
				slotHex: "0x04",
				writeVal: nil,
				want: zeroHash.Bytes(),
			},
			{
				name: "bad-block",
				target: contract,
				slotHex: "0x05",
				writeVal: rlpEncodedValue,
				expectErr: storage.ErrLayerNotFound,	
			},
		}

		for i, tc := range testSuite{
			var blockHash ctypes.Hash
			if tc.expectErr != nil{
				blockHash = crypto.Keccak256Hash(newAccounts(1)[0].addr.Bytes())
			} else{
				header := &types.Header{
					Number: big.NewInt(int64(i + 1)),
					ParentHash: genesisHash,
					Difficulty: big.NewInt(int64(i + 1)), 
					GasLimit: gasLimit,
				}
				rawHeader,_ := rlp.EncodeToBytes(header)
				blockHash = crypto.Keccak256Hash(rawHeader)

				updates := []storage.KeyValue{
					{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
				}

				if tc.writeVal != nil {
					slotHash := ctypes.HexToHash(tc.slotHex)
					hashedSlot := crypto.Keccak256Hash(slotHash.Bytes())

					updates = append(updates, storage.KeyValue{
						Key: schema.AccountStorage(chainID, tc.target.Bytes(), hashedSlot.Bytes()), Value: tc.writeVal,
					})
				}

				sdb.Storage.AddBlock(blockHash,
					header.ParentHash,
					header.Number.Uint64(),
					header.Difficulty,
					updates, nil,
					[]byte(header.Number.Text(10)),
				)
			}

			result, err := e.GetStorageAt(rpc.NewContext(context.Background()), tc.target, tc.slotHex, vm.BlockNumberOrHash{BlockHash: &blockHash})
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %v: want error %v, have nothing", i, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr){
					if err.Error() != tc.expectErr.Error() {
						t.Fatalf("%s: expected %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Fatalf("%v: unexpected error: %v", tc.name, err)
			}
			resBytes, ok := result.(hexutil.Bytes)
			if !ok {
				t.Fatalf("unexpected result type: %T", result)
			}
			if !bytes.Equal(resBytes, tc.want) {
				t.Fatalf("error in eth_storageAt, %v: storage mismatch expected: %v actual : %v", tc.name, tc.want, resBytes)
			}
		}

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

	// t.Run("Ethercattle_EstimateGasList", func(t *testing.T){
	// 	accounts := newAccounts(2)
	// 	from, to := accounts[0].addr, accounts[1].addr 

	// 	fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
	// 	toAccount := state.Account{Balance: big.NewInt(0)}
	// 	encodedFrom, _ := rlp.EncodeToBytes(fromAccount)
	// 	encodedTo, _ := rlp.EncodeToBytes(toAccount)

	// 	header := &types.Header{
	// 		Number: big.NewInt(1),
	// 		ParentHash: genesisHash,
	// 		Difficulty: big.NewInt(2), 
	// 	}
	// 	rawHeader,_ := rlp.EncodeToBytes(header)
	// 	blockhash := crypto.Keccak256Hash(rawHeader)

	// 	updates := []storage.KeyValue{
	// 		{ Key: schema.BlockHeader(chainID, blockhash.Bytes()), Value: rawHeader},
	// 		{ Key: schema.AccountData(chainID, from.Bytes()), Value: encodedFrom},
	// 		{ Key: schema.AccountData(chainID, to.Bytes()), Value: encodedTo},
	// 	}

	// 	sdb.Storage.AddBlock(blockhash,
	// 		genesisHash, 
	// 		header.Number.Uint64(),
	// 		header.Difficulty, updates, nil, 
	// 		[]byte("1"),
	// 	)

	// 	gas := hexutil.Uint64(100000)
	// 	args := []TransactionArgs{
	// 		{
	// 			From: &from,
	// 			To: &to,
	// 			Gas: &gas,
	// 		},
	// 	}
	// 	precise := true
	// 	res, err := ethercattleApi.EstimateGasList(rpc.NewContext(context.Background()), args, &precise)
	// 	if err != nil {
	// 		t.Fatal(err.Error())
	// 	}
	// 	if len(res) != 1 || res[0] != hexutil.Uint64(params.TxGas) {
	// 		t.Fatalf("error in estimateGas, expected [2100], got %s", res)
	// 	}
	// })

	t.Run("EstimateGas", func(t *testing.T){
		accounts := newAccounts(2)
		from,to := accounts[0].addr, accounts[1].addr 

		fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
		toAccount := state.Account{Balance: big.NewInt(0)}
		encodedFrom, _ := rlp.EncodeToBytes(fromAccount)
		encodedTo, _ := rlp.EncodeToBytes(toAccount)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2), 
			GasLimit: gasLimit,
		}

		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainID, from.Bytes()), Value: encodedFrom},
			{Key: schema.AccountData(chainID, to.Bytes()),  Value: encodedTo},
		}

		sdb.Storage.AddBlock(blockHash,
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates, nil,
			[]byte("1"),
		)

		gas := hexutil.Uint64(100000)      
		args := TransactionArgs{
			From: &from,
			To:   &to,
			Gas:  &gas,
		}

		res, err := e.EstimateGas(rpc.NewContext(context.Background()), args, &vm.BlockNumberOrHash{BlockHash: &blockHash})
		if err != nil {
			t.Fatalf(err.Error())
		}
		if res != hexutil.Uint64(params.TxGas) {
			t.Fatalf("error in estimateGas mismatch: expected %d, actual %d", params.TxGas, res)
		}
	})

	t.Run("SendRawTransaction", func(t *testing.T){
		accounts := newAccounts(2)
		from, to := accounts[0], accounts[1]

		fromAccount := state.Account{Balance: big.NewInt(params.Ether), Nonce: 0}
		toAccount := state.Account{Balance: big.NewInt(0)}
		encodedFrom,_ := rlp.EncodeToBytes(fromAccount)
		encodedTo, _ := rlp.EncodeToBytes(toAccount)

		header := &types.Header{
			Number: big.NewInt(2),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(9),
			GasLimit:   gasLimit, 
		}
		rawHeader,_ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainID, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainID, from.addr.Bytes()), Value: encodedFrom},
			{Key: schema.AccountData(chainID, to.addr.Bytes()), Value: encodedTo},
		}

		sdb.Storage.AddBlock(blockHash, 
			header.ParentHash, 
			header.Number.Uint64(),
			header.Difficulty, 
			updates, nil,
			[]byte("1"),
		)

		txData := &types.LegacyTx{
			Nonce: 0,
			To: &to.addr,
			Value: big.NewInt(100),
			Gas: params.TxGas,
			GasPrice: big.NewInt(params.Wei),  
			Data: nil,
		}

		tx := types.NewTx(txData)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(chainID)), from.key)
		if err!=nil {
			t.Fatalf("signing error %v", err)
		}
		rawBytes,err := signedTx.MarshalBinary()
		if err!= nil {
			t.Fatalf("error marshalling binary %v", err)
		}

		emitter  := &mockEmitter{}
		poolAPI := NewPublicTransactionPoolAPI(emitter, mgr)

		hash, err := poolAPI.SendRawTransaction(rpc.NewContext(context.Background()), rawBytes)
		if err!=nil {
			t.Fatalf(err.Error())
		}

		if signedTx.Hash() != hash {
			t.Fatalf("error in SendRawTransaction, hash mismatch. expected %v, got %v", signedTx.Hash(), hash)
		}
		if emitter.seen == nil {
			t.Fatal("emitter did not receive any transaction")
		}
		if emitter.seen.Hash() != signedTx.Hash() {
			t.Fatalf("emitted tx hash mismatch: got %s, want %s",
				emitter.seen.Hash(), signedTx.Hash())
		}
	})


	t.Run("Web3_ClientVersion", func(t *testing.T) {
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
