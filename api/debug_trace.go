package api

import (
	"fmt"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/tracers"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"
)

// PublicDebugTraceAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicDebugTraceAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

type TraceCallConfig struct {
	vm.LogConfig
	Tracer         *string
	Timeout        *string
	Reexec         *uint64
	StateOverrides *StateOverride
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}


// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewDebugTraceAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PublicDebugTraceAPI {
	return &PublicDebugTraceAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}



func (api *PublicDebugTraceAPI) TraceCall(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash vm.BlockNumberOrHash, config *TraceCallConfig) (interface{}, error) {
	var res interface{}
	err := api.evmmgr.View(blockNrOrHash, args.From, &vm.Config{NoBaseFee: true}, ctx, func(statedb state.StateDB, header *types.Header, evmFn func(state.StateDB, *vm.Config, common.Address) *vm.EVM) error {
		if err := config.StateOverrides.Apply(statedb); err != nil {
			return err
		}
		msg, err := args.ToMessage(header.GasLimit*2, header.BaseFee)
		if err != nil {
			return err
		}
		var traceConfig *TraceConfig
		var tracer tracers.TracerResult
		var ok bool
		if config != nil {
			traceConfig = &TraceConfig{
				LogConfig:  config.LogConfig,
				Tracer:  config.Tracer,
				Timeout: config.Timeout,
				Reexec:  config.Reexec,
			}
			if config.Tracer == nil {
				x := "default"
				config.Tracer = &x
			}

			tracer, ok = tracers.Get(*config.Tracer, statedb, &config.LogConfig)
		} else {
			tracer, ok = tracers.Get("default", statedb, nil)
		}
		if !ok {
			return fmt.Errorf("tracer not found")
		}
		noop(msg, traceConfig, tracer)
		return nil
	})
	return res, err
}

func noop(...interface{}) {}


// func (api *PublicDebugTraceAPI) traceApi()
