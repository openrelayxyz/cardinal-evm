package api

import (
	"math/big"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

type EtherCattleBlockChainAPI struct {
	evmmgr *vm.EVMManager
	gasLimit RPCGasLimit
}

func NewEtherCattleBlockChainAPI(evmmgr *vm.EVMManager, gasLimit RPCGasLimit) *EtherCattleBlockChainAPI {
	return &EtherCattleBlockChainAPI{evmmgr: evmmgr, gasLimit: gasLimit}
}

// EstimateGasList returns an estimate of the amount of gas needed to execute list of
// given transactions against the current pending block.
func (s *EtherCattleBlockChainAPI) EstimateGasList(ctx *rpc.CallContext, argsList []TransactionArgs, precise *bool) ([]hexutil.Uint64, error) {
	fast := precise == nil || !*precise
	blockNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	returnVals := make([]hexutil.Uint64, len(argsList))
	err := s.evmmgr.View(blockNrOrHash, &vm.Config{NoBaseFee: true}, ctx, func(statedb state.StateDB, header *types.Header, evmFn func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM) error {
		var (
			gas       hexutil.Uint64
			err       error
			stateData = &PreviousState{statedb, header}
			gasCap    = s.gasLimit(header)
		)
		for idx, args := range argsList {
			args.normalize()
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
	if err != nil {
		switch err.(type) {
		case codedError:
		default:
			err = evmError{err}
		}
	}
	return returnVals, err
}
