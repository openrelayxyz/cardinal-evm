package api

import (
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

type EtherCattleBlockChainAPI struct {
	evmmgr *vm.EVMManager
}

func NewEtherCattleBlockChainAPI(evmmgr *vm.EVMManager) *EtherCattleBlockChainAPI {
	return &EtherCattleBlockChainAPI{evmmgr}
}

// EstimateGasList returns an estimate of the amount of gas needed to execute list of
// given transactions against the current pending block.
func (s *EtherCattleBlockChainAPI) EstimateGasList(ctx *rpc.CallContext, argsList []TransactionArgs, precise *bool) ([]hexutil.Uint64, error) {
	fast := precise == nil || !*precise
	blockNrOrHash := vm.BlockNumberOrHashWithNumber(vm.PendingBlockNumber)
	returnVals := make([]hexutil.Uint64, len(argsList))
	err := s.evmmgr.View(blockNrOrHash, &vm.Config{NoBaseFee: true}, ctx, func(statedb state.StateDB, header *types.Header, evmFn func(state.StateDB, *vm.Config, common.Address) *vm.EVM) error {
		var (
			gas       hexutil.Uint64
			err       error
			stateData = &PreviousState{statedb, header}
			gasCap    = header.GasLimit * 2
		)
		for idx, args := range argsList {
			gas, stateData, err = DoEstimateGas(ctx, evmFn, args, stateData, blockNrOrHash, gasCap, fast)
			// DoEstimateGas(ctx, s.b, args, stateData, blockNrOrHash, gasCap, fast)
			if err != nil {
				return err
			}
			gasCap -= uint64(gas)
			returnVals[idx] = gas
		}
		return nil
	})
	return returnVals, err
}
