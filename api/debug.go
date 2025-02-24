package api

import (
	"math/big"
	"fmt"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/params"
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
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), nil, nil, false)

	_, err = ApplyMessage(getEVM(db.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from(), msg.GasPrice()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	return tracer.StructLogs(), nil
}
