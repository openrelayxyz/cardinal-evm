package api

import (
	// "encoding/json"
	"math/big"
	"fmt"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/api/tracers"
	"github.com/openrelayxyz/cardinal-rpc"
)

type PrivateDebugAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewDebugAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PrivateDebugAPI {
	return &PrivateDebugAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}

type UnsupportedFeature struct{}

// TODO: Think about ways to find the field name for the error?
func (*UnsupportedFeature) UnmarshalJSON(d []byte) error {
	if len(d) > 0 && string(d) != "null" {
		return fmt.Errorf("unsupported field specified")
	}
	return nil
}

type PublicDebugAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewPublicDebugAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PublicDebugAPI {
	return &PublicDebugAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}



func (api *PublicDebugAPI) TraceCall(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash, config *tracers.TraceCallConfig) (interface{}, error) {
	if config.StateOverrides != nil {
		return nil, fmt.Errorf("state overrides not supported")
	}
	if config.BlockOverrides != nil {
		return nil, fmt.Errorf("block overrides not supported")
	}
	if config.Overrides != nil {
		return nil, fmt.Errorf("overrides not supported")
	}
	if config.Debug {
		return nil, fmt.Errorf("debug logging not supported")
	}
	var tracer tracers.TracerResult
	if config.Tracer != nil {
		tracerFn, ok := tracers.Registry[*config.Tracer]
		if !ok {
			return nil, fmt.Errorf("only builtin tracers are supported")
		}
		t, err := tracerFn(&tracers.Context{}, config.TracerConfig)
		if err != nil {
			return nil, err
		}
		tracer = t

	} else {
		tracer = vm.NewStructLogger(&vm.LogConfig{
			DisableMemory: !config.EnableMemory,
			DisableStack: config.DisableStack,
			DisableStorage: config.DisableStorage,
			DisableReturnData: !config.EnableReturnData,
			Debug: false, // This would control printing to console, which we don't want to expose to public
			Limit:  config.Limit,
		})
	}
	err := api.evmmgr.View(blockNrOrHash, args.From, ctx, func(header *types.Header, statedb state.StateDB, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chaincfg *params.ChainConfig) error {
		var err error

		if err := args.setDefaults(ctx, getEVM, statedb, header, vm.BlockNumberOrHashWithNumber(rpc.BlockNumber(header.Number.Int64()))); err != nil {
			return err
		}

		value := args.Value.ToInt()
		if value == nil { value = new(big.Int) }
		gasPrice := args.GasPrice.ToInt()
		if gasPrice == nil { gasPrice = new(big.Int) }
		msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), nil, false)

		_, err = ApplyMessage(getEVM(statedb.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from(), msg.GasPrice()), msg, new(GasPool).AddGas(msg.Gas()))
		if err != nil {
			return fmt.Errorf("failed to apply transaction: err: %v", err)
		}
		
		return err
	})
	return tracer.Result(), err
}

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
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), nil, false)

	_, err = ApplyMessage(getEVM(db.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from(), msg.GasPrice()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	return tracer.StructLogs(), nil
}
