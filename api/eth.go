// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package api

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	// "strings"
	"time"

	"github.com/openrelayxyz/cardinal-evm/abi"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/common/math"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	log "github.com/inconshreveable/log15"
)

// PublicBlockChainAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewETHAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}

// ChainId is the EIP-155 replay-protection chain id for the current ethereum chain config.
func (api *PublicBlockChainAPI) ChainId() *hexutil.Big {
	return (*hexutil.Big)(new(big.Int).SetInt64(api.chainid))
}

// BlockNumber returns the block number of the chain head.
func (api *PublicBlockChainAPI) BlockNumber(ctx *rpc.CallContext) (hexutil.Uint64, error) {
	var result hexutil.Uint64
	if err := api.evmmgr.View(ctx, func(num uint64) {
		result = hexutil.Uint64(num)
	}); err != nil {
		return hexutil.Uint64(0), err
	}
	return result, nil
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx *rpc.CallContext, address common.Address, blockNrOrHash vm.BlockNumberOrHash) (*hexutil.Big, error) {
	var result *hexutil.Big
	if err := s.evmmgr.View(blockNrOrHash, ctx, func(statedb state.StateDB) {
		result = (*hexutil.Big)(statedb.GetBalance(address))
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx *rpc.CallContext, address common.Address, blockNrOrHash vm.BlockNumberOrHash) (hexutil.Bytes, error) {
	var result hexutil.Bytes
	if err := s.evmmgr.View(blockNrOrHash, ctx, func(statedb state.StateDB) {
		result = hexutil.Bytes(statedb.GetCode(address))
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx *rpc.CallContext, address common.Address, key string, blockNrOrHash vm.BlockNumberOrHash) (hexutil.Bytes, error) {
	var result hexutil.Bytes
	if err := s.evmmgr.View(blockNrOrHash, ctx, func(statedb state.StateDB) {
		result = hexutil.Bytes(statedb.GetState(address, ctypes.HexToHash(key)).Bytes())
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// OverrideAccount indicates the overriding fields of account during the execution
// of a message call.
// Note, state and stateDiff can't be specified at the same time. If state is
// set, message execution will only use the data in the given state. Otherwise
// if statDiff is set, all diff will be applied first and then execute the call
// message.
type OverrideAccount struct {
	Nonce     *hexutil.Uint64              `json:"nonce"`
	Code      *hexutil.Bytes               `json:"code"`
	Balance   **hexutil.Big                `json:"balance"`
	State     *map[ctypes.Hash]ctypes.Hash `json:"state"`
	StateDiff *map[ctypes.Hash]ctypes.Hash `json:"stateDiff"`
}

type PreviousState struct {
	state  state.StateDB
	header *types.Header
}

func (prevState *PreviousState) copy() *PreviousState {
	if prevState == nil {
		return nil
	}
	state := prevState.state
	if state != nil {
		state = state.Copy()
	}
	return &PreviousState{
		state:  state,
		header: prevState.header,
	}
}

// StateOverride is the collection of overridden accounts.
type StateOverride map[common.Address]OverrideAccount

// Apply overrides the fields of specified accounts into the given state.
func (diff *StateOverride) Apply(state state.StateDB) error {
	if diff == nil {
		return nil
	}
	for addr, account := range *diff {
		// Override account nonce.
		if account.Nonce != nil {
			state.SetNonce(addr, uint64(*account.Nonce))
		}
		// Override account(contract) code.
		if account.Code != nil {
			state.SetCode(addr, *account.Code)
		}
		// Override account balance.
		if account.Balance != nil {
			state.SetBalance(addr, (*big.Int)(*account.Balance))
		}
		if account.State != nil && account.StateDiff != nil {
			return fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr.Hex())
		}
		// Replace entire state if caller requires.
		if account.State != nil {
			state.SetStorage(addr, *account.State)
		}
		// Apply state diff into specified accounts.
		if account.StateDiff != nil {
			for key, value := range *account.StateDiff {
				state.SetState(addr, key, value)
			}
		}
	}
	return nil
}

func DoCall(cctx *rpc.CallContext, getEVM func(state.StateDB, *vm.Config, common.Address) *vm.EVM, args TransactionArgs, prevState *PreviousState, blockNrOrHash vm.BlockNumberOrHash, overrides *StateOverride, timeout time.Duration, globalGasCap uint64) (*ExecutionResult, *PreviousState, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())
	if prevState == nil || prevState.header == nil || prevState.state == nil {
		return nil, nil, fmt.Errorf("both header and state must be set")
	}
	if err := overrides.Apply(prevState.state); err != nil {
		return nil, nil, err
	}
	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	var ctx context.Context
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(cctx.Context(), timeout)
	} else {
		ctx, cancel = context.WithCancel(cctx.Context())
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	msg, err := args.ToMessage(globalGasCap, prevState.header.BaseFee)
	if err != nil {
		return nil, nil, err
	}
	evm := getEVM(prevState.state, nil, args.from())
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Execute the message.
	gp := new(GasPool).AddGas(math.MaxUint64)
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, nil, err
	}
	cctx.Metadata().AddCompute(result.UsedGas)

	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		return nil, nil, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}
	if err != nil {
		return result, nil, fmt.Errorf("err: %w (supplied gas %d)", err, msg.Gas())
	}
	prevState.state.Finalise()
	return result, prevState, err
}

func newRevertError(result *ExecutionResult) *revertError {
	reason, errUnpack := abi.UnpackRevert(result.Revert())
	err := errors.New("execution reverted")
	if errUnpack == nil {
		err = fmt.Errorf("execution reverted: %v", reason)
	}
	return &revertError{
		error:  err,
		reason: hexutil.Encode(result.Revert()),
	}
}

// revertError is an API error that encompassas an EVM revertal with JSON error
// code and a binary data blob.
type revertError struct {
	error
	reason string // revert reason hex encoded
}

// ErrorCode returns the JSON error code for a revertal.
// See: https://github.com/ethereum/wiki/wiki/JSON-RPC-Error-Codes-Improvement-Proposal
func (e *revertError) ErrorCode() int {
	return 3
}

// ErrorData returns the hex encoded revert reason.
func (e *revertError) ErrorData() interface{} {
	return e.reason
}

// Call executes the given transaction on the state for the given block number.
//
// Additionally, the caller can specify a batch of contract for fields overriding.
//
// Note, this function doesn't make and changes in the state/blockchain and is
// useful to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash vm.BlockNumberOrHash, overrides *StateOverride) (hexutil.Bytes, error) {
	var timeout time.Duration
	if args.Gas != nil {
		timeout = time.Duration(*args.Gas/10000000) * time.Second
	}
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	var res hexutil.Bytes
	err := s.evmmgr.View(blockNrOrHash, args.From, &vm.Config{NoBaseFee: true}, ctx, func(statedb state.StateDB, header *types.Header, evmFn func(state.StateDB, *vm.Config, common.Address) *vm.EVM) error {
		result, _, err := DoCall(ctx, evmFn, args, &PreviousState{statedb, header}, blockNrOrHash, overrides, timeout, header.GasLimit*2)
		if err != nil {
			return err
		}
		// If the result contains a revert reason, try to unpack and return it.
		if len(result.Revert()) > 0 {
			return newRevertError(result)
		}
		log.Debug("EVM result", "result", result)
		res = result.Return()
		return result.Err
	})
	return res, err
}

//
type estimateGasError struct {
	error  string // Concrete error type if it's failed to estimate gas usage
	vmerr  error  // Additional field, it's non-nil if the given transaction is invalid
	revert string // Additional field, it's non-empty if the transaction is reverted and reason is provided
}

func (e estimateGasError) Error() string {
	errMsg := e.error
	if e.vmerr != nil {
		errMsg += fmt.Sprintf(" (%v)", e.vmerr)
	}
	if e.revert != "" {
		errMsg += fmt.Sprintf(" (%s)", e.revert)
	}
	return errMsg
}

func DoEstimateGas(ctx *rpc.CallContext, getEVM func(state.StateDB, *vm.Config, common.Address) *vm.EVM, args TransactionArgs, prevState *PreviousState, blockNrOrHash vm.BlockNumberOrHash, gasCap uint64, approx bool) (hexutil.Uint64, *PreviousState, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo        uint64 = params.TxGas - 1
		hi        uint64
		cap       uint64
		stateData *PreviousState
	)
	if prevState == nil || prevState.header == nil || prevState.state == nil {
		return 0, nil, fmt.Errorf("both header and state must be set")
	}
	// Use zero address if sender unspecified.
	if args.From == nil {
		args.From = new(common.Address)
	}
	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Retrieve the block to act as the gas ceiling
		hi = prevState.header.GasLimit
	}
	// Recap the highest gas limit with account's available balance.
	if args.GasPrice != nil && args.GasPrice.ToInt().BitLen() != 0 {
		balance := prevState.state.GetBalance(*args.From)
		available := new(big.Int).Set(balance)
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available) >= 0 {
				return 0, nil, errors.New("insufficient funds for transfer")
			}
			available.Sub(available, args.Value.ToInt())
		}
		allowance := new(big.Int).Div(available, args.GasPrice.ToInt())

		// If the allowance is larger than maximum uint64, skip checking
		if allowance.IsUint64() && hi > allowance.Uint64() {
			transfer := args.Value
			if transfer == nil {
				transfer = new(hexutil.Big)
			}
			log.Warn("Gas estimation capped by limited funds", "original", hi, "balance", balance,
				"sent", transfer.ToInt(), "gasprice", args.GasPrice.ToInt(), "fundable", allowance)
			hi = allowance.Uint64()
		}
	}
	// Recap the highest gas allowance with specified gascap.
	if gasCap != 0 && hi > gasCap {
		log.Warn("Caller gas above allowance, capping", "requested", hi, "cap", gasCap)
		hi = gasCap
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, *ExecutionResult, error) {
		args.Gas = (*hexutil.Uint64)(&gas)

		result, prevS, err := DoCall(ctx, getEVM, args, prevState.copy(), blockNrOrHash, nil, 0, gasCap)
		if prevS != nil && !result.Failed() {
			stateData = prevS
		}
		if err != nil {
			if errors.Is(err, ErrIntrinsicGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return result.Failed(), result, nil
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		failed, _, err := executable(mid)

		// If the error is not nil(consensus error), it means the provided message
		// call or transaction will never be accepted no matter how much gas it is
		// assigned. Return the error directly, don't struggle any more.
		if err != nil {
			return 0, nil, err
		}
		if failed {
			lo = mid
		} else {
			hi = mid
		}
		if approx && (hi-lo) < (hi/100) {
			break
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		failed, result, err := executable(hi)
		if err != nil {
			return 0, nil, err
		}
		if failed {
			if result != nil && result.Err != vm.ErrOutOfGas {
				if len(result.Revert()) > 0 {
					return 0, nil, newRevertError(result)
				}
				return 0, nil, result.Err
			}
			// Otherwise, the specified gas cap is too low
			return 0, nil, fmt.Errorf("gas required exceeds allowance (%d)", cap)
		}
	}
	return hexutil.Uint64(hi), stateData, nil
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockChainAPI) EstimateGas(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) (hexutil.Uint64, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(vm.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	var gas hexutil.Uint64
	err := s.evmmgr.View(bNrOrHash, args.From, &vm.Config{NoBaseFee: true}, ctx, func(statedb state.StateDB, header *types.Header, evmFn func(state.StateDB, *vm.Config, common.Address) *vm.EVM) error {
		var err error
		gas, _, err = DoEstimateGas(ctx, evmFn, args, &PreviousState{statedb, header}, bNrOrHash, header.GasLimit*2, false)
		return err
	})
	return gas, err
}

// accessListResult returns an optional accesslist
// Its the result of the `debug_createAccessList` RPC call.
// It contains an error if the transaction itself failed.
type accessListResult struct {
	Accesslist *types.AccessList `json:"accessList"`
	Error      string            `json:"error,omitempty"`
	GasUsed    hexutil.Uint64    `json:"gasUsed"`
}

// CreateAccessList creates a EIP-2930 type AccessList for the given transaction.
// Reexec and BlockNrOrHash can be specified to create the accessList on top of a certain state.
func (s *PublicBlockChainAPI) CreateAccessList(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) (*accessListResult, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(vm.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	var result *accessListResult
	err := s.evmmgr.View(bNrOrHash, args.From, ctx, func(header *types.Header, statedb state.StateDB, evmFn func(state.StateDB, *vm.Config, common.Address) *vm.EVM, chaincfg *params.ChainConfig) error {
		acl, gasUsed, vmerr, err := AccessList(ctx, statedb, header, chaincfg, evmFn, bNrOrHash, args)
		if err != nil {
			return err
		}
		result = &accessListResult{Accesslist: &acl, GasUsed: hexutil.Uint64(gasUsed)}
		if vmerr != nil {
			result.Error = vmerr.Error()
		}
		return nil
	})
	return result, err
}

// AccessList creates an access list for the given transaction.
// If the accesslist creation fails an error is returned.
// If the transaction itself fails, an vmErr is returned.
func AccessList(ctx *rpc.CallContext, db state.StateDB, header *types.Header, chaincfg *params.ChainConfig, getEVM func(state.StateDB, *vm.Config, common.Address) *vm.EVM, blockNrOrHash vm.BlockNumberOrHash, args TransactionArgs) (acl types.AccessList, gasUsed uint64, vmErr error, err error) {
	noGas := args.Gas == nil
	if err := args.setDefaults(ctx, getEVM, db, header, blockNrOrHash); err != nil {
		return nil, 0, nil, err
	}
	var to common.Address
	if args.To != nil {
		to = *args.To
	} else {
		to = crypto.CreateAddress(args.from(), uint64(*args.Nonce))
	}
	// Retrieve the precompiles since they don't need to be added to the access list
	precompiles := vm.ActivePrecompiles(chaincfg.Rules(header.Number))

	// Create an initial tracer
	tracer := vm.NewAccessListTracer(nil, args.from(), to, precompiles)
	if args.AccessList != nil {
		tracer = vm.NewAccessListTracer(*args.AccessList, args.from(), to, precompiles)
	}
	// Get a copy of the statedb primed for calculating the access list
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), args.Value.ToInt(), uint64(*args.Gas), args.GasPrice.ToInt(), big.NewInt(0), big.NewInt(0), args.data(), tracer.AccessList(), false)

	_, err = ApplyMessage(getEVM(db.ALCalcCopy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	tracer = vm.NewAccessListTracer(tracer.AccessList(), args.from(), to, precompiles)

	gas := uint64(*args.Gas)
	if noGas {
		// In fairly simple transactions, gas estimations tend to be low because
		// the `ACCESS_LIST_ADDRESS_COST` (EIP-2930) is higher than any gas savings
		// from the access list. The difference will (almost?) always be < 2400, so
		// adding 2400 will ensure we have enough gas for the next call to
		// complete.
		gas += 2400
	}
	msg = NewMessage(args.from(), args.To, uint64(*args.Nonce), args.Value.ToInt(), uint64(*args.Gas)+2400, args.GasPrice.ToInt(), big.NewInt(0), big.NewInt(0), args.data(), tracer.AccessList(), false)
	res, err := ApplyMessage(getEVM(db.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	return tracer.AccessList(), res.UsedGas, res.Err, nil
}

type TransactionEmitter interface {
	Emit(*types.Transaction) error
}

type PublicTransactionPoolAPI struct {
	emitter TransactionEmitter
	evmmgr  *vm.EVMManager
}

func NewPublicTransactionPoolAPI(emitter TransactionEmitter, evmmgr *vm.EVMManager) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{emitter, evmmgr}
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx *rpc.CallContext, input hexutil.Bytes) (ctypes.Hash, error) {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(input); err != nil {
		return ctypes.Hash{}, err
	}
	return tx.Hash(), s.evmmgr.View(func(currentState state.StateDB, header *types.Header, chaincfg *params.ChainConfig) error {
		if ctx != nil {
			if err := ctx.Context().Err(); err != nil {
				return err
			}
		}
		if s.emitter == nil {
			return errors.New("This api is not configured for accepting transactions")
		}
		msg, err := tx.AsMessage(types.MakeSigner(chaincfg, header.Number), header.BaseFee)
		if err != nil {
			return err
		}
		if n := currentState.GetNonce(msg.From()); n > tx.Nonce() {
			return ErrNonceTooLow
		}

		// Check the transaction doesn't exceed the current
		// block limit gas.
		if header.GasLimit < tx.Gas() {
			return ErrGasLimit
		}

		// Transactions can't be negative. This may never happen
		// using RLP decoded transactions but may occur if you create
		// a transaction using the RPC for example.
		if tx.Value().Sign() < 0 {
			return ErrNegativeValue
		}

		// Transactor should have enough funds to cover the costs
		// cost == V + GP * GL
		if b := currentState.GetBalance(msg.From()); b.Cmp(tx.Cost()) < 0 {
			return ErrInsufficientFunds
		}

		// Should supply enough intrinsic gas
		gas, err := IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil, true, chaincfg.IsIstanbul(header.Number))
		if err != nil {
			return err
		}
		if tx.Gas() < gas {
			return ErrIntrinsicGas
		}
		return s.emitter.Emit(tx)
	})
}
