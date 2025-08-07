package api

import (
	"fmt"
	"encoding/json"
	"math/big"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	ctypes "github.com/openrelayxyz/cardinal-types"
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

// TraceContext provides context for tracing
type TraceContext struct {
	BlockHash   ctypes.Hash
	BlockNumber *big.Int
	TxIndex     int
	TxHash      ctypes.Hash
}

const defaultTraceReexec = uint64(128)

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

	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}

	var result interface{}
	err := s.evmmgr.View(bNrOrHash, args.from, ctx, func(header *types.Header, statedb state.StateDB, evmFn func (state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chaincfg *params.ChainConfig) error{
		if config != nil {
			if err := config.StateOverrides.Apply(statedb); err != nil {
				return err
			}
		}

		gasCap := new(GasPool).AddGas(header.GasLimit)
		if err := args.CallDefaults(gasCap.Gas(), header.BaseFee, chaincfg.ChainID); err != nil {
			return err
		}

		msg, err := args.ToMessage(gasCap.Gas(), header.BaseFee) 
		if err != nil {
			return err
		}

		tx := args.ToTransaction(types.DynamicFeeTxType)
		txctx := &TraceContext{
			BlockHash:   header.Hash(),
			BlockNumber: header.Number,
			TxIndex:     0, 
			TxHash:      tx.Hash(),
		}


		if msg.gasPrice.Sign() == 0 {
			header.BaseFee = new(big.Int)
		}

		var traceConfig *TraceConfig
		if config != nil {
			traceConfig = &config.TraceConfig
		}
	

		result, err = s.traceTx(ctx, tx, msg, txctx, header, statedb, traceConfig)
		return err
	})

	if err != nil {
		switch err.(type) {
		case codedError:
		default:
			err = evmError{err}
		}
		return nil, err
	}
	return result, nil
}

func (s *PrivateDebugAPI) traceTx(ctx *rpc.CallContext, tx *types.Transaction, message Msg, txctx *TraceContext, header *types.Header, statedb state.StateDB, config *TraceConfig) (interface{}, error){

}