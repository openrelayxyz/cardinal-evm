package api

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/openrelayxyz/cardinal-evm/api/tracers"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"

	// ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

type PrivateDebugAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
	// Config specific to given tracer. Note struct logger
	// config are historically embedded in main object.
	TracerConfig json.RawMessage
}

// TraceCallConfig is the config for traceCall API. It holds one more
// field to override the state for tracing.
type TraceCallConfig struct {
	TraceConfig
	StateOverrides *StateOverride
	TxIndex        *hexutil.Uint
}


// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewDebugAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PrivateDebugAPI {
	return &PrivateDebugAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}

// CreateAccessList creates a EIP-2930 type AccessList for the given transaction.
// Reexec and BlockNrOrHash can be specified to create the accessList on top of a certain state.
func (s *PrivateDebugAPI) TraceStructLog(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) ([]vm.StructLog, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	var result []vm.StructLog
	err := s.evmmgr.View(bNrOrHash, args.From, ctx, func(header *types.Header, statedb state.StateDB, evmFn func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chaincfg *params.ChainConfig) error {
		var err error
		result, err = TraceStructLog(ctx, statedb, header, chaincfg, evmFn, bNrOrHash, args)
		return err
	})
	if err != nil {
		switch err.(type) {
		case codedError:
		default:
			err = evmError{err}
		}
	}
	return result, err
}

// AccessList creates an access list for the given transaction.
// If the accesslist creation fails an error is returned.
// If the transaction itself fails, an vmErr is returned.
func TraceStructLog(ctx *rpc.CallContext, db state.StateDB, header *types.Header, chaincfg *params.ChainConfig, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, blockNrOrHash vm.BlockNumberOrHash, args TransactionArgs) (slog []vm.StructLog, err error) {
	if err := args.setDefaults(ctx, getEVM, db, header, blockNrOrHash); err != nil {
		return nil, err
	}
	// Create an initial tracer
	tracer := vm.NewStructLogger(&vm.LogConfig{})
	// Get a copy of the statedb primed for calculating the access list
	value := args.Value.ToInt()
	if value == nil { value = new(big.Int) }
	gasPrice := args.GasPrice.ToInt()
	if gasPrice == nil { gasPrice = new(big.Int) }
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), *args.AccessList, args.AuthList, false)

	_, err = ApplyMessage(getEVM(db.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from(), msg.GasPrice()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	return tracer.StructLogs(), nil
}

// TraceCall lets you trace a given eth_call. It collects the structured logs
// created during the execution of EVM if the given transaction was added on
// top of the provided block and returns them as a JSON object.
// If no transaction index is specified, the trace will be conducted on the state
// after executing the specified block. However, if a transaction index is provided,
// the trace will be conducted on the state after executing the specified transaction
// within the specified block.
func (s *PrivateDebugAPI) TraceCall(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash, config *TraceCallConfig) (interface{}, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}

	var (
		statedb state.StateDB
		header *types.Header
		chaincfg *params.ChainConfig
	)

	if config != nil  && config.TxIndex != nil {
		var err error
		statedb, header, chaincfg, err = s.stateAtTransaction(ctx, bNrOrHash, int(*config.TxIndex))
		if err != nil {
			return nil, err
		}
	} else {
		err := s.evmmgr.View(bNrOrHash, args.from, ctx, func(h *types.Header, sdb state.StateDB, evmFunc func (state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, cfg *params.ChainConfig) error{
			statedb = sdb
			header = h
			chaincfg = cfg
			return nil
		})
		if err != nil {
			switch err.(type) {
			case codedError:
			default:
				err = evmError{err}
			}
			return nil, err
		}
	}

	if config != nil {
		if err := config.StateOverrides.Apply(statedb); err != nil {
			return nil, err
		}
	}

	gasCap := new(GasPool).AddGas(header.GasLimit)
	if err := args.CallDefaults(gasCap.Gas(), header.BaseFee, chaincfg.ChainID); err != nil {
		return nil, err
	}

	msg, err := args.ToMessage(gasCap.Gas(), header.BaseFee) 
	if err != nil {
		return nil, err
	}

	tx := args.ToTransaction(types.DynamicFeeTxType)

	if msg.gasPrice.Sign() == 0 {
		header.BaseFee = new(big.Int)
	}

	var traceConfig *TraceConfig
	if config != nil {
		traceConfig = &config.TraceConfig
	}
	return s.traceTx(ctx, tx, msg, new(tracers.Context), header, statedb, traceConfig)
}

func (s *PrivateDebugAPI) traceTx(ctx *rpc.CallContext, tx *types.Transaction, message Msg, txctx *tracers.Context, header *types.Header, statedb state.StateDB, config *TraceConfig) (interface{}, error){

}

func (s *PrivateDebugAPI) stateAtTransaction(ctx *rpc.CallContext, blockNrOrHash vm.BlockNumberOrHash, txIndex int) (state.StateDB, *types.Header, *params.ChainConfig, error){
	err := s.evmmgr.View(blockNrOrHash, common.Address{}, ctx, func(header *types.Header, statedb state.StateDB, evmFn func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chainCfg *params.ChainConfig) error {
		blockHash := header.Hash()
		if hash, ok := blockNrOrHash.Hash(); ok {
			blockHash = hash
		}

		var transactions []*types.Transaction
		err := s.storage.View(blockHash, func(tx storage.Transaction) error {
			for i := 0; ; i++ {
				txkey := schema.Transaction(s.chainid, blockHash.Bytes(), int64(i))
				txdata, err := tx.Get(txkey)
				if err != nil{ break }

				var transaction types.Transaction
				if err := rlp.DecodeBytes(txdata, &transaction); err != nil {
					return fmt.Errorf("failed to decode transaction %v: %v", i, err)
				}
				transactions = append(transactions, &transaction)
			}
			return nil
		})
		if err!=nil{
			return err
		}

		if txIndex >= len(transactions) {
			return fmt.Errorf("transaction index %d exceeds block transaction count %d", txIndex, len(transactions))
		}
	})
}