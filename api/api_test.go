package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"reflect"
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
	"github.com/holiman/uint256"
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
	web3Api :=&Web3API{}

	t.Run("BlockNumber", func (t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		block1Hash := ctypes.HexToHash("0x01")
		sdb.Storage.AddBlock(
			block1Hash,
			genesisHash, 
			1, 
			big.NewInt(1), nil, nil, 
			[]byte("1"),
		)
	
		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})
		res, err := e.BlockNumber(rpc.NewContext(context.Background()))
		if err != nil {
			t.Fatal(err.Error())
		}
		if res != 1 {
			t.Errorf("Blocknumber result not accurate")
		}
	})

	t.Run("GetBalance", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		addr := newAccounts(1)[0].addr
		bal := big.NewInt(42)

		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})

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
					{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
				}

				if tc.balance != nil {
					encodedAccount, _ := rlp.EncodeToBytes(state.Account{Balance: tc.balance})
					updates = append(updates, storage.KeyValue{
						Key: schema.AccountData(chainId, addr.Bytes()), Value: encodedAccount,
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
						t.Errorf("%v: expected error %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			   }
				if err!=nil{
					t.Error(err.Error())
				}
				if(*big.Int)(result).Cmp(tc.want) != 0 {
					t.Errorf("error in getBalance, %s: balance mismatch. expected %s, got %s", tc.name, tc.want.String(), (*big.Int)(result).String())
				}
		}
	})

	t.Run("GetCode", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		accounts := newAccounts(1)
		address := accounts[0].addr

		// ConfigurableValue
		contractA := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80633fa4f24514602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212201ac5f57785e601674d20159494ae12fb2cc62ec0a9152929e20dfa835723007b64736f6c634300081e0033"))

		// DoubleStore
		contractB := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80636d4ce63c14602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea264697066735822122021f2db8e56187d1ecda9ce08724e0a64379f499aa7551783058fe1cc8812512364736f6c634300081e0033"))
		
		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})
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
					{ Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader },
				}

				if tc.code != nil {
					var contractHash ctypes.Hash 
					
					if len(tc.code) > 0 {
						contractHash = crypto.Keccak256Hash(tc.code)
						updates = append(updates, storage.KeyValue{
							Key: schema.AccountCode(chainId, contractHash.Bytes()),
							Value: tc.code,
						})
					} else{
						contractHash = crypto.Keccak256Hash(nil)
					}

					contractAcct := state.Account{CodeHash: contractHash.Bytes()}
					encodedContract, _ := rlp.EncodeToBytes(contractAcct)
					updates = append(updates, storage.KeyValue{
						Key: schema.AccountData(chainId, address.Bytes()),
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
						t.Errorf("%s: expected %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Error(err.Error())
			}
			resBytes, ok := result.(hexutil.Bytes)
			if !ok {
				t.Errorf("unexpected result type %T", result )
			}

			if !bytes.Equal(resBytes, tc.want){
				t.Errorf("error in getCode, %v : expected %v, actual %v",  tc.name, tc.want, resBytes.String())
			}
		}
	})

	t.Run("Call", func(t *testing.T){
		mgr, _, sdb , chainId := setupEVMTest(t)

		accounts := newAccounts(6)
		from, to := accounts[0].addr, accounts[1].addr
		wei1000 := (*hexutil.Big)(big.NewInt(1000))

		genesisHeader := &types.Header{
			Number: big.NewInt(0),
			GasLimit: gasLimit,
		}
		rawGenHeader, _ := rlp.EncodeToBytes(genesisHeader)
		genesisHash := crypto.Keccak256Hash(rawGenHeader)

		genesisUpdates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainId, genesisHash.Bytes()), Value: rawGenHeader},
			{Key: schema.AccountData(chainId, from.Bytes()), Value: fund(params.Ether)},
			{Key: schema.AccountData(chainId, to.Bytes()), Value: fund(params.Ether)},
			{Key: schema.AccountData(chainId, accounts[2].addr.Bytes()), Value: fund(0)},
		}

		sdb.Storage.AddBlock(genesisHash, 
			ctypes.Hash{}, 0, 
			big.NewInt(0), 
			genesisUpdates, nil, 
			[]byte("0"),
		)

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2),
			GasLimit: gasLimit,
		}
		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		sdb.Storage.AddBlock(blockHash, 
			header.ParentHash, 
			header.Number.Uint64(), 
			header.Difficulty, 
			[]storage.KeyValue{
				{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
			}, nil, 
			[]byte("1"),
		)

		txGas := hexutil.Uint64(params.CallNewAccountGas)

		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})

		var testSuite = [] struct {
			name string
			blockNumber rpc.BlockNumber
			call TransactionArgs
			overrides StateOverride
			want string	
			expectErr error
		}{
			{
				name: "transfer-on-genesis",
				blockNumber: rpc.BlockNumber(0),
				call: TransactionArgs{
					From: &from,
					To: &to,
					Value: wei1000,
				},
				expectErr: nil,
				want: "0x",
			},
			{
				name: "transfer-latest-block",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &to,
					Value: wei1000,
				},
				expectErr: nil,
				want:      "0x",
			},
			{
				name: "transfer-non-existent-block",
				blockNumber: rpc.BlockNumber(99),
				call: TransactionArgs{
					From: &from,
					To: &to,
					Value: wei1000,
				},
				// hardcoding this for the main time
				expectErr: errors.New("error getting block header: Not Found (c/1/b/0000000000000000000000000000000000000000000000000000000000000000/h)"),
			},
			{
				name: "state-override-success",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From:  &from,
					To:    &to,
					Value: wei1000,
				},
				overrides: StateOverride{
					from: {Balance: newRPCBalance(big.NewInt(params.Ether))} ,
				},
				want: "0x",
			},
			{
				name: "insufficient-funds-simple",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &accounts[3].addr,
					To: &accounts[4].addr,
					Value: wei1000,
				},
				expectErr: fmt.Errorf("%v: address %v", ErrInsufficientFundsForTransfer, accounts[3].addr),
			},
			// ConfigurableValue()
			{
				name: "simple-contract-call",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &accounts[5].addr,
					Data: hex2Bytes("3fa4f245"), // function value()
				},
				overrides: StateOverride{
					accounts[5].addr: { 
						Code: hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c80633fa4f24514602a575b5f5ffd5b5f5460405190815260200160405180910390f3fea26469706673582212201ac5f57785e601674d20159494ae12fb2cc62ec0a9152929e20dfa835723007b64736f6c634300081e0033"),
						StateDiff: &map[ctypes.Hash]ctypes.Hash{{}: ctypes.BigToHash(big.NewInt(123))},
					},
				},
				want: "0x000000000000000000000000000000000000000000000000000000000000007b",
			},
			{
				name: "contract-revert",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &accounts[5].addr,
					Data: hex2Bytes("a9cc4718"),
				},
				overrides: StateOverride{
					accounts[5].addr: { 
						Code: hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c8063a9cc471814602a575b5f5ffd5b60306032565b005b60405162461bcd60e51b815260040160629060208082526004908201526319985a5b60e21b604082015260600190565b60405180910390fdfea26469706673582212203f4611ac62517a213443b59f76a7f45aa4e4a060b4cf965d1fe3c8a90ba8af8164736f6c634300081e0033"),
					},
				},
				expectErr: errors.New("execution reverted: fail"),
			},
			{
				name: "call-EOA-empty-code",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &accounts[2].addr,
					Data: hex2Bytes("3fa4f245"),
				},
				want: "0x",
			},
			{
				name: "call-precompile-identity",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &common.Address{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x04},
					Data: hex2Bytes("deadbeef"),
					Gas: &txGas,
				},
				want: "0xdeadbeef",
			},
		}

		for _, tc := range testSuite {
			result, err := e.Call(rpc.NewContext(context.Background()), tc.call, vm.BlockNumberOrHash{BlockNumber: &tc.blockNumber}, &tc.overrides)
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %s: want error %v, have nothing", tc.name, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr) {
					if err.Error() != tc.expectErr.Error() {
						t.Errorf("test %s: error mismatch, want %v, have %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Errorf("test:%v, err: %v", tc.name, err)
				continue
			}
			resBytes := result.(hexutil.Bytes)
			if !reflect.DeepEqual(resBytes.String(), tc.want) {
				t.Errorf("error in eth_call, %v expected %s, got %s", tc.name, tc.want, resBytes.String())
			}
		}
	})

	t.Run("CreateAccessList", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		accounts := newAccounts(2)
		from, to := accounts[0].addr, accounts[1].addr

		// simpleStorage
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
			{ Key: schema.BlockHeader(chainId, blockhash.Bytes()), Value: rawHeader},
			{ Key: schema.AccountData(chainId, from.Bytes()), Value: encodedFrom},
			{ Key: schema.AccountData(chainId, to.Bytes()), Value: encodedTo},
			{ Key: schema.AccountCode(chainId, codeHash.Bytes()), Value: contract},
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

		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})
		result, err := e.CreateAccessList(rpc.NewContext(context.Background()), args, &vm.BlockNumberOrHash{BlockHash: &blockhash})
		if err != nil {
			t.Errorf(err.Error())
		}
		if result.Error != "" {
			t.Errorf("CreateAccessList returned a VM error: %v", result.Error)
		}
		if result.Accesslist == nil {
			t.Error("expected an access list, but got nil")
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
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		creator := newAccounts(1)[0].addr
		contract := crypto.CreateAddress(creator, 0)

		storageValue := ctypes.HexToHash("0xdeadbeef") 
		rlpEncodedValue, _ := rlp.EncodeToBytes(storageValue.Bytes())
		zeroHash := ctypes.Hash{} 
		rlpZero, _ := rlp.EncodeToBytes(zeroHash.Bytes())

		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})

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
					{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
				}

				if tc.writeVal != nil {
					slotHash := ctypes.HexToHash(tc.slotHex)
					hashedSlot := crypto.Keccak256Hash(slotHash.Bytes())

					updates = append(updates, storage.KeyValue{
						Key: schema.AccountStorage(chainId, tc.target.Bytes(), hashedSlot.Bytes()), Value: tc.writeVal,
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
						t.Errorf("%s: expected %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Errorf("%v: unexpected error: %v", tc.name, err)
			}
			resBytes, ok := result.(hexutil.Bytes)
			if !ok {
				t.Errorf("unexpected result type: %T", result)
			}
			if !bytes.Equal(resBytes, tc.want) {
				t.Errorf("error in eth_storageAt, %v: storage mismatch expected: %v actual : %v", tc.name, tc.want, resBytes)
			}
		}
	})
	
	t.Run("Debug_TraceStructLog", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		// TracerTest contract
		contract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106030575f3560e01c80633fa4f2451460345780635601eaea14604d575b5f5ffd5b603b5f5481565b60405190815260200160405180910390f35b605c6058366004606e565b605e565b005b5f60678284608d565b5f55505050565b5f5f60408385031215607e575f5ffd5b50508035926020909101359150565b8082018082111560ab57634e487b7160e01b5f52601160045260245ffd5b9291505056fea2646970667358221220cb70395670a2ececa377ddb2a0d7d0d24b58421678f45e6a00fa6416d23cee7664736f6c634300081e0033"))

		// simple revert
		revertContract := hexutil.Bytes(ctypes.Hex2Bytes("6080604052348015600e575f5ffd5b50600436106026575f3560e01c8063a9cc471814602a575b5f5ffd5b60306032565b005b60405162461bcd60e51b815260040160629060208082526004908201526319985a5b60e21b604082015260600190565b60405180910390fdfea26469706673582212203f4611ac62517a213443b59f76a7f45aa4e4a060b4cf965d1fe3c8a90ba8af8164736f6c634300081e0033"))

		data := hexutil.Bytes(append(ctypes.Hex2Bytes("5601eaea"), ctypes.LeftPadBytes(big.NewInt(10).Bytes(), 32)...))
		data = append(data, ctypes.LeftPadBytes(big.NewInt(20).Bytes(), 32)...)
		revertdata := hexutil.Bytes(ctypes.Hex2Bytes("a9cc4718"))

		debugApi := NewDebugAPI(sdb.Storage, mgr, chainId, defaultTraceTimeout)

		var testSuite = []struct {
			name string
			contract []byte 
			data *hexutil.Bytes
			wantRevert bool
			expectErr error
		}{
			{
				name: "addition-contract",
				contract:contract,
				data: &data,
				wantRevert: false,
			},
			{
				name: "contract-revert",
				contract: revertContract,
				data: &revertdata,
				wantRevert: true,
			},
			{
				name: "bad-block",
				contract: contract,
				data:       &data,
				expectErr:  storage.ErrLayerNotFound,
			},
		}

		for i, tc := range testSuite {
			accounts := newAccounts(2)
			from, to := accounts[0].addr, accounts[1].addr 

			codeHash := crypto.Keccak256Hash(tc.contract)
			fromAccount := state.Account{Balance: big.NewInt(params.Ether)}
			toAccount := state.Account{CodeHash: codeHash.Bytes()}

			encodedFrom,_ := rlp.EncodeToBytes(fromAccount)
			encodedTo,_ := rlp.EncodeToBytes(toAccount)

			var blockHash ctypes.Hash

			if tc.expectErr != nil {
				blockHash = crypto.Keccak256Hash(newAccounts(1)[0].addr.Bytes())
			}else {
				header := &types.Header{
					Number: big.NewInt(int64(i + 3)),
					ParentHash: genesisHash,
					Difficulty: big.NewInt(1),
					GasLimit:   gasLimit, 
				}
				rawHeader,_ := rlp.EncodeToBytes(header)
				blockHash = crypto.Keccak256Hash(rawHeader)

				updates := []storage.KeyValue{
					{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
					{Key: schema.AccountData(chainId, from.Bytes()),       Value: encodedFrom},
					{Key: schema.AccountData(chainId, to.Bytes()),         Value: encodedTo},
					{Key: schema.AccountCode(chainId, codeHash.Bytes()),   Value: tc.contract},
				}

				sdb.Storage.AddBlock(blockHash,
					header.ParentHash,
					header.Number.Uint64(),
					header.Difficulty,
					updates, nil,
					[]byte(header.Number.Text(10)),
				)
			}

			gas := hexutil.Uint64(150000)

			args := TransactionArgs{
				From: &from,
				To: &to,
				Gas: &gas,
				Data: tc.data,
			}
	
			logs, err := debugApi.TraceStructLog(rpc.NewContext(context.Background()), args, &vm.BlockNumberOrHash{BlockHash: &blockHash})
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %d: want error %v, have nothing", i, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr){
					if err.Error() != tc.expectErr.Error() {
						t.Errorf("%s: expected %v, got %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err!=nil {
				t.Errorf(err.Error())
			}
			if len(logs) == 0 {
				t.Errorf("%v: empty Trace", tc.name)
			}
			last := logs[len(logs)-1]

			if tc.wantRevert {
				var sawRevert bool
				for _, l := range logs {
					if l.Op == vm.REVERT {
						sawRevert = true
						break
					}
				}
				if !sawRevert {
					t.Errorf("%s: REVERT opcode not found", tc.name)
				}
				continue 
			} else if last.Err != nil {
				t.Errorf("%s: unexpected error at end: %v", tc.name, last.Err)
			}
			
			// check addition success
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
				t.Errorf("error in debug_tracestructlog, Add(10,20) not found or stack wrong")
			}
			if !storeOk {
				t.Errorf("error in debug_tracestructlog, SSTORE(slot 0, 30) not found or stack wrong")
			}
			if logs[len(logs)-1].Err != nil {
				t.Errorf("error in debug_tracestructlog, trace ended with error: %v", logs[len(logs)-1].Err)
			}
			
		}
	})

	t.Run("Ethercattle_EstimateGasList", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)
		b := func(v bool) *bool { return &v }

		accounts := newAccounts(3)
		from, to := accounts[0].addr, accounts[1].addr 

		ethercattleApi := NewEtherCattleBlockChainAPI(mgr, func(*types.Header) uint64 {return gasLimit})

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2), 
			GasLimit: gasLimit,
		}
		rawHeader,_ := rlp.EncodeToBytes(header)
		blockhash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{ Key: schema.BlockHeader(chainId, blockhash.Bytes()), Value: rawHeader},
			{ Key: schema.AccountData(chainId, from.Bytes()), Value: fund(params.Ether)},
			{ Key: schema.AccountData(chainId, to.Bytes()), Value: fund(0)},
			{ Key: schema.AccountData(chainId, accounts[2].addr.Bytes()), Value: fund(0)}, 
		}

		sdb.Storage.AddBlock(blockhash,
			genesisHash, 
			header.Number.Uint64(),
			header.Difficulty, updates, nil, 
			[]byte("1"),
		)
		gasCap := hexutil.Uint64(100_000)

		var testSuite = []struct {
			name string
			args [] TransactionArgs
			precise *bool
			wantValues []uint64
			expectErr error
		}{
			{
				name: "single-transfer",
				args: []TransactionArgs{
					{
						From: &from,
						To: &to,
						Gas: func() *hexutil.Uint64 { g := hexutil.Uint64(100000); return &g }(),
					},
				},
				precise: func() *bool { p := true; return &p }(),
				wantValues: []uint64{params.TxGas},
			},
			{
				name: "multiple-transfers",
				args: []TransactionArgs{
					{
						From: &from,
						To: &to,
						Value: (*hexutil.Big)(big.NewInt(100)),
						Gas: &gasCap,
					},
					{
						From: &from,
						To: &accounts[2].addr,
						Value: (*hexutil.Big)(big.NewInt(200)),
						Gas: &gasCap,
					},
				},
				precise: b(false), 
				wantValues: []uint64{21113, 21113},   
			},
			{
				name: "empty-list",
				args: []TransactionArgs{},
				precise: b(true), 
				wantValues: []uint64{},
			},
		}

		for _, tc := range testSuite{
			res, err := ethercattleApi.EstimateGasList(rpc.NewContext(context.Background()), tc.args, tc.precise)
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %s: want error %v, have nothing", tc.name, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr) {
					if err.Error() != tc.expectErr.Error() {
						t.Errorf("test %s: error mismatch, want %v, have %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Errorf("test %s: unexpected error: %v", tc.name, err)
				continue
			}
			if len(res) != len(tc.wantValues) {
				t.Errorf("test %s: length mismatch, expected %d, got %d", tc.name, len(tc.wantValues), len(res))
				continue
			}
			
			for i, expected := range tc.wantValues {
				if i >= len(res) {
					t.Errorf("test %s: missing result at index %d", tc.name, i)
					break
				}
				if float64(res[i]) > float64(expected)*(1+estimateGasErrorRatio) {
					t.Errorf("test %s: gas estimate mismatch at index %d, expected %d, got %d", tc.name, i, expected, uint64(res[i]))
				}
			}
		}
	})

	t.Run("EstimateGas", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		accounts := newAccounts(6)
		from,to := accounts[0].addr, accounts[1].addr 

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2), 
			GasLimit: gasLimit,
			BaseFee: big.NewInt(1_000_000_000),
		}

		rawHeader, _ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainId, from.Bytes()), Value: fundWithNonce(params.Ether, 0)},
			{Key: schema.AccountData(chainId, to.Bytes()),  Value: fund(0)},
			{Key: schema.AccountData(chainId, accounts[4].addr.Bytes()), Value: fundWithCode(params.Ether, types.AddressToDelegation(accounts[5].addr))},
		}

		sdb.Storage.AddBlock(blockHash,
			header.ParentHash,
			header.Number.Uint64(),
			header.Difficulty,
			updates, nil,
			[]byte("1"),
		)
		wei1000 := (*hexutil.Big)(big.NewInt(1000))

		e := NewETHAPI(sdb.Storage, mgr, chainId, func(*types.Header)uint64{ return gasLimit})

		 
		auth := types.Authorization{
			ChainID: *uint256.NewInt(uint64(chainId)),
			Address: accounts[0].addr, 
			Nonce:   1, 
		}
		signedAuth, _ := types.SignAuth(auth, accounts[0].key)

		var testSuite = []struct {
			name string
			blockNumber rpc.BlockNumber
			call TransactionArgs
			overrides StateOverride
			want uint64	
			expectErr error
		}{
			{
				name: "simple-transfer-on-latest",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &to,
					Value: wei1000,
				},
				expectErr: nil,
				want: params.TxGas,
			},
			{
				name: "insuffienct-funds",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &accounts[2].addr,
					To: &accounts[1].addr,
					Value: wei1000,
				},
				expectErr: fmt.Errorf("%v: address %v", ErrInsufficientFundsForTransfer, accounts[2].addr),
			},
			{
				name: "empty-create",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{},
				expectErr: nil,
				want: params.TxGasContractCreation,
			},
			{
				name: "empty-create-with-override",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{},
				overrides: StateOverride{
					accounts[2].addr : { Balance: newRPCBalance(big.NewInt(params.Ether))},
				},
				expectErr: nil,
				want: params.TxGasContractCreation,
			},
			{
				name: "override-insufficient-funds",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &accounts[2].addr,
					To: &accounts[3].addr,
					Value: wei1000,
				},
				overrides: StateOverride{
					accounts[3].addr: {Balance: newRPCBalance(big.NewInt(0))},
				},
				expectErr: fmt.Errorf("%v: address %v", ErrInsufficientFundsForTransfer, accounts[2].addr),
			},

			// BaseFeeChecker
			{
				name: "legacy-gasprice",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From:     &from,
					Input:    hex2Bytes("6080604052348015600e575f5ffd5b50483a1015601a575f5ffd5b3a156029575f48116029575f5ffd5b603e8060345f395ff3fe60806040525f5ffdfea26469706673582212200e6c49b7fc5bc8d827f135b2931af68aa6df0c79de95619bd4843d3b4daad0fb64736f6c634300081e0033"),
					GasPrice: (*hexutil.Big)(big.NewInt(params.GWei)), 
				},
				expectErr: nil,
				want:      67617,
			},
			{
				name: "eip1559-gas-pricing",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					Input: hex2Bytes("6080604052348015600e575f5ffd5b50483a1015601a575f5ffd5b3a156029575f48116029575f5ffd5b603e8060345f395ff3fe60806040525f5ffdfea26469706673582212200e6c49b7fc5bc8d827f135b2931af68aa6df0c79de95619bd4843d3b4daad0fb64736f6c634300081e0033"),
					MaxFeePerGas: (*hexutil.Big)(big.NewInt(params.GWei)), // 1559 gas pricing
				},
				expectErr: nil,
				want: 67617,
			},
			{
				name: "default-gas-pricing",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &accounts[0].addr,
					Input:  hex2Bytes("6080604052348015600e575f5ffd5b50483a1015601a575f5ffd5b3a156029575f48116029575f5ffd5b603e8060345f395ff3fe60806040525f5ffdfea26469706673582212200e6c49b7fc5bc8d827f135b2931af68aa6df0c79de95619bd4843d3b4daad0fb64736f6c634300081e0033"),
					GasPrice: nil, // No legacy gas pricing
					MaxFeePerGas: nil, // No 1559 gas pricing
				},
				expectErr: vm.ErrExecutionReverted, // Cardinal doesn't set default,
			},
			{
				name:"send-to-delegated-account",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &accounts[4].addr,
					Value:(*hexutil.Big)(big.NewInt(1)),
				},
				want: params.TxGas,
			},
			{
				name:"send-from-delegated-account",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From:&accounts[4].addr,
					To: &accounts[1].addr,
					Value: (*hexutil.Big)(big.NewInt(1)),
				},
				want: 21000,
			},
			{
				name: "setcode-authorization",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &accounts[2].addr,
					To: &accounts[3].addr,
					Value: (*hexutil.Big)(big.NewInt(0)),
					AuthList: []types.Authorization{signedAuth},
				},
				want: 46000,
			},
			{
				name:"setcode-invalid-opcode",
				blockNumber: rpc.LatestBlockNumber,
				call: TransactionArgs{
					From: &from,
					To: &from,
					Value: (*hexutil.Big)(big.NewInt(0)),
					AuthList: []types.Authorization{signedAuth},
				},
				expectErr: errors.New("invalid opcode: opcode 0xef not defined"),
			},
		}

		for _, tc := range testSuite {
			result, err := e.EstimateGas(rpc.NewContext(context.Background()), tc.call, &vm.BlockNumberOrHash{BlockNumber: &tc.blockNumber})
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %s: want error %v, have nothing", tc.name, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr) {
					if err.Error() != tc.expectErr.Error() {
						t.Errorf("test %s: error mismatch, want %v, have %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Errorf("test:%v, err: %v", tc.name, err)
				continue
			}
			if float64(result) > float64(tc.want)*(1+estimateGasErrorRatio) {
				t.Errorf("error in estimateGas. %v mismatch: expected %d, got %d", tc.name, tc.want, result)
			}
		}
	})

	t.Run("SendRawTransaction", func(t *testing.T){
		mgr, _, sdb, chainId := setupEVMTest(t)
		genesisHash := addGenesis(sdb, chainId)

		accounts := newAccounts(4)
		from, to := accounts[0], accounts[1]

		header := &types.Header{
			Number: big.NewInt(1),
			ParentHash: genesisHash,
			Difficulty: big.NewInt(2),
			GasLimit:   gasLimit, 
			BaseFee:    big.NewInt(1_000_000_000),
		}
		rawHeader,_ := rlp.EncodeToBytes(header)
		blockHash := crypto.Keccak256Hash(rawHeader)

		updates := []storage.KeyValue{
			{Key: schema.BlockHeader(chainId, blockHash.Bytes()), Value: rawHeader},
			{Key: schema.AccountData(chainId, from.addr.Bytes()), Value: fund(params.Ether)},
			{Key: schema.AccountData(chainId, to.addr.Bytes()), Value: fund(0)},
			{Key: schema.AccountData(chainId, accounts[2].addr.Bytes()), Value: fund(1000)},
			{Key: schema.AccountData(chainId, accounts[3].addr.Bytes()), Value: fundWithNonce(params.Ether, 5)},
		}

		sdb.Storage.AddBlock(blockHash, 
			header.ParentHash, 
			header.Number.Uint64(),
			header.Difficulty, 
			updates, nil,
			[]byte("1"),
		)

		emitter  := &mockEmitter{}
		poolAPI := NewPublicTransactionPoolAPI(emitter, mgr)

		var testSuite = []struct{
			name string 
			buildTx func() ([]byte, ctypes.Hash)
			expectErr error
		}{
			{
				name: "legacy-transfer-success",
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.LegacyTx{
						Nonce:0,
						To:&to.addr,
						Value: big.NewInt(100),
						Gas:params.TxGas,
						GasPrice: big.NewInt(params.GWei),
					}
					tx := types.NewTx(txData)
					signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(chainId)), from.key)
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: nil,
			},
			{
				name: "insufficient-funds",
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.LegacyTx{
						Nonce:  0,
						To: &to.addr,
						Value: big.NewInt(params.Ether), 
						Gas: params.TxGas,
						GasPrice: big.NewInt(params.GWei),
					}
					tx := types.NewTx(txData)
					signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(chainId)), accounts[2].key)
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: ErrInsufficientFunds,
			},
			{
				name: "invalid-signature",
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.LegacyTx{
						Nonce:0,
						To: &to.addr,
						Value:big.NewInt(100),
						Gas: params.TxGas,
						GasPrice: big.NewInt(params.GWei),
					}
					tx := types.NewTx(txData)

					signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(999)), from.key) // wrong chainID
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: types.ErrInvalidChainId,
			},
			{
				name: "nonce-too-low", 
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.LegacyTx{
						Nonce: 2, 
						To:  &to.addr,
						Value: big.NewInt(100),
						Gas: params.TxGas,
						GasPrice: big.NewInt(params.GWei),
					}
					tx := types.NewTx(txData)
					signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(chainId)), accounts[3].key)
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: ErrNonceTooLow,
			},
			{
				name: "eip1559-transfer",
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.DynamicFeeTx{
						ChainID: big.NewInt(chainId),
						Nonce: 0,
						To: &to.addr,
						Value: big.NewInt(100),
						Gas: params.TxGas,
						GasTipCap: big.NewInt(2_000_000_000), 
						GasFeeCap: big.NewInt(3_000_000_000),
					}
					tx := types.NewTx(txData)
					signedTx, _ := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainId)), from.key)
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: nil,
			},
			{
				name: "intrinsic-gas-too-low",
				buildTx: func() ([]byte, ctypes.Hash) {
					txData := &types.LegacyTx{
						Nonce:    0,
						To:       &to.addr,
						Value:    big.NewInt(100),
						Gas:      20000, 
						GasPrice: big.NewInt(params.GWei),
					}
					tx := types.NewTx(txData)
					signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(chainId)), from.key)
					rawBytes, _ := signedTx.MarshalBinary()
					return rawBytes, signedTx.Hash()
				},
				expectErr: ErrIntrinsicGas,
			},
		}

		for _, tc := range testSuite{
			emitter.seen = nil
			rawBytes, expectedHash := tc.buildTx()

			hash, err := poolAPI.SendRawTransaction(rpc.NewContext(context.Background()), rawBytes)
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("test %v: want error %v, have nothing", tc.name, tc.expectErr)
					continue
				}
				if !errors.Is(err, tc.expectErr) {
					if err.Error() != tc.expectErr.Error() {
						t.Errorf("test %v: error mismatch, want %v, have %v", tc.name, tc.expectErr, err)
					}
				}
				continue
			}
			if err != nil {
				t.Errorf("test:%v, err: %v", tc.name, err)
				continue
			}
			if hash != expectedHash {
				t.Errorf("hash mismatch: expected %v, got %v", expectedHash, hash)
			}
			
			if emitter.seen == nil {
				t.Error("emitter did not receive transaction")
			}
			if emitter.seen.Hash() != expectedHash {
				t.Errorf("emitted tx hash mismatch: got %v, want %v", emitter.seen.Hash(), expectedHash)
			}
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
			t.Errorf("invalid version format: %s", version)
		}
	})
}
