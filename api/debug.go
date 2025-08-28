package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/openrelayxyz/cardinal-evm/api/tracers"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

// defaultTraceTimeout is the amount of time a single transaction can execute
// by default before being forcefully aborted.
const	defaultTraceTimeout = 5 * time.Second

type PrivateDebugAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
	traceTimeout time.Duration 
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
	// Config specific to given tracer. Note struct logger
	// config are historically embedded in main object.x
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
func NewDebugAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64, traceTimeout time.Duration) *PrivateDebugAPI {
	if traceTimeout == 0 {
        traceTimeout = defaultTraceTimeout  
    }
	return &PrivateDebugAPI{storage: s, evmmgr: evmmgr, chainid: chainid, traceTimeout: traceTimeout}
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
	var acl types.AccessList
	if args.AccessList != nil {
		acl = *args.AccessList
	}
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), acl, args.AuthList, false)

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

	if config != nil  && config.TxIndex != nil  && *config.TxIndex != 0{
		return nil, fmt.Errorf("txIndex %d not supported: only supports txIndex 0", *config.TxIndex)
	} 

	var result interface {}
	err := s.evmmgr.View(bNrOrHash, args.From, ctx, func(header *types.Header, statedb state.StateDB, evmFn func (state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, cfg *params.ChainConfig) error{
		if config != nil && config.StateOverrides != nil {
			if err := config.StateOverrides.Apply(statedb); err != nil {
				return err
			}
		}
		gasCap := new(GasPool).AddGas(header.GasLimit)
		if err := args.setDefaults(ctx, evmFn, statedb, header, bNrOrHash); err != nil {
			return err
		}
		msg, err := args.ToMessage(gasCap.Gas(), header.BaseFee) 
		if err != nil {
			return err
		}
		tx := args.ToTransaction(types.LegacyTxType)
		if msg.GasPrice().Sign() == 0 {
			header.BaseFee = new(big.Int)
		}
		var traceConfig *TraceConfig
		if config != nil {
			traceConfig = &config.TraceConfig
		}
		result, err = s.traceTx(ctx, tx, msg, new(tracers.Context), header, statedb, evmFn, cfg, traceConfig)
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

// traceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (s *PrivateDebugAPI) traceTx(ctx *rpc.CallContext, tx *types.Transaction, message Msg, txctx *tracers.Context, header *types.Header, statedb state.StateDB, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chainConfig *params.ChainConfig, config *TraceConfig) (interface{}, error){
	var ( 
		timeout = s.traceTimeout
		err error
	)

	if config == nil {
		config = &TraceConfig{}
	}
	if config.Timeout != nil {
		if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
			return nil, err
		}
	}
	var tracer vm.Tracer
	if config == nil || config.Tracer == nil {
		 tracer = vm.NewStructLogger(&vm.LogConfig{})
	} else {
		tracer, err = tracers.New(*config.Tracer, config.TracerConfig, chainConfig)
		if err != nil {
            return nil, err
        }
		if stateinjector, ok := tracer.(tracers.StateInjector); ok{
			vmCtx := &tracers.VMContext{
				Coinbase:    header.Coinbase,
				BlockNumber: header.Number,
				Time:        header.Time,
				StateDB:     statedb,
			}
			stateinjector.SetVMContext(vmCtx)
		}
	}
	deadlineCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
	defer cancel()
	evm := getEVM(statedb, &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, message.From(), message.GasPrice())

	go func() {
		<-deadlineCtx.Done()
		if deadlineCtx.Err() == context.DeadlineExceeded {
			evm.Cancel()
		}
	}()

	statedb.SetTxContext(txctx.TxHash, txctx.TxIndex)
	_, err = ApplyMessage(evm, message, new(GasPool).AddGas(header.GasLimit))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	if evm.Cancelled() {
		return nil, fmt.Errorf("execution timeout")
	}

	return tracer.GetResult()
}
