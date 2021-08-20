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
	// "errors"
	// "fmt"
	"math/big"
	// "strings"
	// "time"

	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-evm/common"
	// "github.com/openrelayxyz/cardinal-evm/common/math"
	"github.com/openrelayxyz/cardinal-evm/state"
	// "github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	// "github.com/openrelayxyz/cardinal-evm/crypto"
	// log "github.com/inconshreveable/log15"
	// "github.com/openrelayxyz/cardinal-evm/params"
	// "github.com/openrelayxyz/cardinal-evm/rlp"
	// "github.com/tyler-smith/go-bip39"
)



// PublicBlockChainAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	storage storage.Storage
	evmmgr     *vm.EVMManager
	chainid int64
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewPublicBlockChainAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}


// ChainId is the EIP-155 replay-protection chain id for the current ethereum chain config.
func (api *PublicBlockChainAPI) ChainId() (*hexutil.Big, error) {
	return (*hexutil.Big)(new(big.Int).SetInt64(api.chainid)), nil
}

// BlockNumber returns the block number of the chain head.
func (api *PublicBlockChainAPI) BlockNumber() (hexutil.Uint64, error) {
	var result hexutil.Uint64
	if err := api.evmmgr.View(func(num uint64) {
		result = hexutil.Uint64(num)
	}); err != nil { return hexutil.Uint64(0), err }
	return result, nil
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash vm.BlockNumberOrHash) (*hexutil.Big, error) {
	var result *hexutil.Big
	if err := s.evmmgr.View(blockNrOrHash, func(statedb state.StateDB) {
		result = (*hexutil.Big)(statedb.GetBalance(address))
	}); err != nil { return nil, err }
	return result, nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash vm.BlockNumberOrHash) (hexutil.Bytes, error) {
	var result hexutil.Bytes
	if err := s.evmmgr.View(blockNrOrHash, func(statedb state.StateDB) {
		result = hexutil.Bytes(statedb.GetCode(address))
	}); err != nil { return nil, err }
	return result, nil
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, key string, blockNrOrHash vm.BlockNumberOrHash) (hexutil.Bytes, error) {
	var result hexutil.Bytes
	if err := s.evmmgr.View(blockNrOrHash, func(statedb state.StateDB) {
		result = hexutil.Bytes(statedb.GetState(address, ctypes.HexToHash(key)).Bytes())
	}); err != nil { return nil, err }
	return result, nil
}
//
// // OverrideAccount indicates the overriding fields of account during the execution
// // of a message call.
// // Note, state and stateDiff can't be specified at the same time. If state is
// // set, message execution will only use the data in the given state. Otherwise
// // if statDiff is set, all diff will be applied first and then execute the call
// // message.
// type OverrideAccount struct {
// 	Nonce     *hexutil.Uint64              `json:"nonce"`
// 	Code      *hexutil.Bytes               `json:"code"`
// 	Balance   **hexutil.Big                `json:"balance"`
// 	State     *map[common.Hash]common.Hash `json:"state"`
// 	StateDiff *map[common.Hash]common.Hash `json:"stateDiff"`
// }
//
// type PreviousState struct {
// 	state  *state.StateDB
// 	header *types.Header
// }
// func (prevState *PreviousState) copy() *PreviousState {
// 	if prevState == nil { return nil }
// 	state := prevState.state
// 	if state != nil { state = state.Copy() }
// 	return &PreviousState{
// 		state: state,
// 		header: prevState.header,
// 	}
// }
//
// // StateOverride is the collection of overridden accounts.
// type StateOverride map[common.Address]OverrideAccount
//
// // Apply overrides the fields of specified accounts into the given state.
// func (diff *StateOverride) Apply(state *state.StateDB) error {
// 	if diff == nil {
// 		return nil
// 	}
// 	for addr, account := range *diff {
// 		// Override account nonce.
// 		if account.Nonce != nil {
// 			state.SetNonce(addr, uint64(*account.Nonce))
// 		}
// 		// Override account(contract) code.
// 		if account.Code != nil {
// 			state.SetCode(addr, *account.Code)
// 		}
// 		// Override account balance.
// 		if account.Balance != nil {
// 			state.SetBalance(addr, (*big.Int)(*account.Balance))
// 		}
// 		if account.State != nil && account.StateDiff != nil {
// 			return fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr.Hex())
// 		}
// 		// Replace entire state if caller requires.
// 		if account.State != nil {
// 			state.SetStorage(addr, *account.State)
// 		}
// 		// Apply state diff into specified accounts.
// 		if account.StateDiff != nil {
// 			for key, value := range *account.StateDiff {
// 				state.SetState(addr, key, value)
// 			}
// 		}
// 	}
// 	return nil
// }
//
// // TODO:
// func DoCall(ctx context.Context, b Backend, args TransactionArgs, prevState *PreviousState, blockNrOrHash vm.BlockNumberOrHash, overrides *StateOverride, timeout time.Duration, globalGasCap uint64) (*core.ExecutionResult, *PreviousState, error) {
// 	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())
// 	if prevState == nil {
// 		prevState = &PreviousState{}
// 	}
// 	if (prevState.header != nil && prevState.header == nil) || (prevState.header == nil && prevState.header != nil) {
// 		return nil, nil, fmt.Errorf("both header and state must be set to use previous staate")
// 	}
// 	if prevState.header == nil && prevState.state == nil {
// 		var err error
// 		prevState.state, prevState.header, err = b.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
// 		if prevState.state == nil || err != nil {
// 			return nil, nil, err
// 		}
// 	}
// 	if err := overrides.Apply(prevState.state); err != nil {
// 		return nil, nil, err
// 	}
// 	// Setup context so it may be cancelled the call has completed
// 	// or, in case of unmetered gas, setup a context with a timeout.
// 	var cancel context.CancelFunc
// 	if timeout > 0 {
// 		ctx, cancel = context.WithTimeout(ctx, timeout)
// 	} else {
// 		ctx, cancel = context.WithCancel(ctx)
// 	}
// 	// Make sure the context is cancelled when the call has completed
// 	// this makes sure resources are cleaned up.
// 	defer cancel()
//
// 	// Get a new instance of the EVM.
// 	msg, err := args.ToMessage(globalGasCap, prevState.header.BaseFee)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	evm, vmError, err := b.GetEVM(ctx, msg, prevState.state, prevState.header, &vm.Config{NoBaseFee: true})
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	// Wait for the context to be done and cancel the evm. Even if the
// 	// EVM has finished, cancelling may be done (repeatedly)
// 	go func() {
// 		<-ctx.Done()
// 		evm.Cancel()
// 	}()
//
// 	// Execute the message.
// 	gp := new(core.GasPool).AddGas(math.MaxUint64)
// 	result, err := core.ApplyMessage(evm, msg, gp)
// 	if err := vmError(); err != nil {
// 		return nil, nil, err
// 	}
//
// 	// If the timer caused an abort, return an appropriate error message
// 	if evm.Cancelled() {
// 		return nil, nil, fmt.Errorf("execution aborted (timeout = %v)", timeout)
// 	}
// 	if err != nil {
// 		return result, nil, fmt.Errorf("err: %w (supplied gas %d)", err, msg.Gas())
// 	}
// 	prevState.header.Root = prevState.state.IntermediateRoot(b.ChainConfig().IsEIP158(prevState.header.Number))
// 	return result, prevState, err
// }
//
// func newRevertError(result *core.ExecutionResult) *revertError {
// 	reason, errUnpack := abi.UnpackRevert(result.Revert())
// 	err := errors.New("execution reverted")
// 	if errUnpack == nil {
// 		err = fmt.Errorf("execution reverted: %v", reason)
// 	}
// 	return &revertError{
// 		error:  err,
// 		reason: hexutil.Encode(result.Revert()),
// 	}
// }
//
// // revertError is an API error that encompassas an EVM revertal with JSON error
// // code and a binary data blob.
// type revertError struct {
// 	error
// 	reason string // revert reason hex encoded
// }
//
// // ErrorCode returns the JSON error code for a revertal.
// // See: https://github.com/ethereum/wiki/wiki/JSON-RPC-Error-Codes-Improvement-Proposal
// func (e *revertError) ErrorCode() int {
// 	return 3
// }
//
// // ErrorData returns the hex encoded revert reason.
// func (e *revertError) ErrorData() interface{} {
// 	return e.reason
// }
//
// // TODO:
// // Call executes the given transaction on the state for the given block number.
// //
// // Additionally, the caller can specify a batch of contract for fields overriding.
// //
// // Note, this function doesn't make and changes in the state/blockchain and is
// // useful to execute and retrieve values.
// func (s *PublicBlockChainAPI) Call(ctx context.Context, args TransactionArgs, blockNrOrHash vm.BlockNumberOrHash, overrides *StateOverride) (hexutil.Bytes, error) {
// 	var timeout time.Duration
// 	if args.Gas != nil {
// 		timeout = time.Duration(*args.Gas / 10000000) * time.Second
// 	}
// 	if timeout < 5 * time.Second {
// 		timeout = 5 * time.Second
// 	}
//
// 	result, _, err := DoCall(ctx, s.b, args, nil, blockNrOrHash, overrides, timeout, s.b.RPCGasCap())
// 	if err != nil {
// 		return nil, err
// 	}
// 	// If the result contains a revert reason, try to unpack and return it.
// 	if len(result.Revert()) > 0 {
// 		return nil, newRevertError(result)
// 	}
// 	return result.Return(), result.Err
// }
//
// type estimateGasError struct {
// 	error  string // Concrete error type if it's failed to estimate gas usage
// 	vmerr  error  // Additional field, it's non-nil if the given transaction is invalid
// 	revert string // Additional field, it's non-empty if the transaction is reverted and reason is provided
// }
//
// func (e estimateGasError) Error() string {
// 	errMsg := e.error
// 	if e.vmerr != nil {
// 		errMsg += fmt.Sprintf(" (%v)", e.vmerr)
// 	}
// 	if e.revert != "" {
// 		errMsg += fmt.Sprintf(" (%s)", e.revert)
// 	}
// 	return errMsg
// }
//
//
// // TODO:
// func DoEstimateGas(ctx context.Context, b Backend, args TransactionArgs, prevState *PreviousState, blockNrOrHash vm.BlockNumberOrHash, gasCap uint64, approx bool) (hexutil.Uint64, *PreviousState, error) {
// 	// Binary search the gas requirement, as it may be higher than the amount used
// 	var (
// 		lo        uint64 = params.TxGas - 1
// 		hi        uint64
// 		cap       uint64
// 		stateData *PreviousState
// 	)
// 	// Use zero address if sender unspecified.
// 	if args.From == nil {
// 		args.From = new(common.Address)
// 	}
// 	// Determine the highest gas limit can be used during the estimation.
// 	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
// 		hi = uint64(*args.Gas)
// 	} else {
// 		// Retrieve the block to act as the gas ceiling
// 		block, err := b.BlockByNumberOrHash(ctx, blockNrOrHash)
// 		if err != nil {
// 			return 0, nil, err
// 		}
// 		if block == nil {
// 			return 0, nil, errors.New("block not found")
// 		}
// 		hi = block.GasLimit()
// 	}
// 	// Recap the highest gas limit with account's available balance.
// 	if args.GasPrice != nil && args.GasPrice.ToInt().BitLen() != 0 {
// 		state, _, err := b.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
// 		if err != nil {
// 			return 0, nil, err
// 		}
// 		balance := state.GetBalance(*args.From) // from can't be nil
// 		if prevState != nil {
// 			balance = prevState.state.GetBalance(*args.From)
// 		}
// 		available := new(big.Int).Set(balance)
// 		if args.Value != nil {
// 			if args.Value.ToInt().Cmp(available) >= 0 {
// 				return 0, nil, errors.New("insufficient funds for transfer")
// 			}
// 			available.Sub(available, args.Value.ToInt())
// 		}
// 		allowance := new(big.Int).Div(available, args.GasPrice.ToInt())
//
// 		// If the allowance is larger than maximum uint64, skip checking
// 		if allowance.IsUint64() && hi > allowance.Uint64() {
// 			transfer := args.Value
// 			if transfer == nil {
// 				transfer = new(hexutil.Big)
// 			}
// 			log.Warn("Gas estimation capped by limited funds", "original", hi, "balance", balance,
// 				"sent", transfer.ToInt(), "gasprice", args.GasPrice.ToInt(), "fundable", allowance)
// 			hi = allowance.Uint64()
// 		}
// 	}
// 	// Recap the highest gas allowance with specified gascap.
// 	if gasCap != 0 && hi > gasCap {
// 		log.Warn("Caller gas above allowance, capping", "requested", hi, "cap", gasCap)
// 		hi = gasCap
// 	}
// 	cap = hi
//
// 	// Create a helper to check if a gas allowance results in an executable transaction
// 	executable := func(gas uint64) (bool, *core.ExecutionResult, error) {
// 		args.Gas = (*hexutil.Uint64)(&gas)
//
// 		result, prevS, err := DoCall(ctx, b, args, prevState.copy(), blockNrOrHash, nil, 0, gasCap)
// 		if prevS != nil && !result.Failed() {
// 			stateData = prevS
// 		}
// 		if err != nil {
// 			if errors.Is(err, core.ErrIntrinsicGas) {
// 				return true, nil, nil // Special case, raise gas limit
// 			}
// 			return true, nil, err // Bail out
// 		}
// 		return result.Failed(), result, nil
// 	}
// 	// Execute the binary search and hone in on an executable gas limit
// 	for lo+1 < hi {
// 		mid := (hi + lo) / 2
// 		failed, _, err := executable(mid)
//
// 		// If the error is not nil(consensus error), it means the provided message
// 		// call or transaction will never be accepted no matter how much gas it is
// 		// assigned. Return the error directly, don't struggle any more.
// 		if err != nil {
// 			return 0, nil, err
// 		}
// 		if failed {
// 			lo = mid
// 		} else {
// 			hi = mid
// 		}
// 		if approx && (hi - lo) < (hi / 100) { break }
// 	}
// 	// Reject the transaction as invalid if it still fails at the highest allowance
// 	if hi == cap {
// 		failed, result, err := executable(hi)
// 		if err != nil {
// 			return 0, nil, err
// 		}
// 		if failed {
// 			if result != nil && result.Err != vm.ErrOutOfGas {
// 				if len(result.Revert()) > 0 {
// 					return 0, nil, newRevertError(result)
// 				}
// 				return 0, nil, result.Err
// 			}
// 			// Otherwise, the specified gas cap is too low
// 			return 0, nil, fmt.Errorf("gas required exceeds allowance (%d)", cap)
// 		}
// 	}
// 	return hexutil.Uint64(hi), stateData, nil
// }
//
// // TODO:
// // EstimateGas returns an estimate of the amount of gas needed to execute the
// // given transaction against the current pending block.
// func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) (hexutil.Uint64, error) {
// 	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
// 	if blockNrOrHash != nil {
// 		bNrOrHash = *blockNrOrHash
// 	}
// 	gas, _, err := DoEstimateGas(ctx, s.b, args, nil, bNrOrHash, s.b.RPCGasCap(), false)
// 	return gas, err
// }
//
// // // ExecutionResult groups all structured logs emitted by the EVM
// // // while replaying a transaction in debug mode as well as transaction
// // // execution status, the amount of gas used and the return value
// // type ExecutionResult struct {
// // 	Gas         uint64         `json:"gas"`
// // 	Failed      bool           `json:"failed"`
// // 	ReturnValue string         `json:"returnValue"`
// // 	StructLogs  []StructLogRes `json:"structLogs"`
// // }
// //
// // // StructLogRes stores a structured log emitted by the EVM while replaying a
// // // transaction in debug mode
// // type StructLogRes struct {
// // 	Pc      uint64             `json:"pc"`
// // 	Op      string             `json:"op"`
// // 	Gas     uint64             `json:"gas"`
// // 	GasCost uint64             `json:"gasCost"`
// // 	Depth   int                `json:"depth"`
// // 	Error   error              `json:"error,omitempty"`
// // 	Stack   *[]string          `json:"stack,omitempty"`
// // 	Memory  *[]string          `json:"memory,omitempty"`
// // 	Storage *map[string]string `json:"storage,omitempty"`
// // }
// //
// // // FormatLogs formats EVM returned structured logs for json output
// // func FormatLogs(logs []vm.StructLog) []StructLogRes {
// // 	formatted := make([]StructLogRes, len(logs))
// // 	for index, trace := range logs {
// // 		formatted[index] = StructLogRes{
// // 			Pc:      trace.Pc,
// // 			Op:      trace.Op.String(),
// // 			Gas:     trace.Gas,
// // 			GasCost: trace.GasCost,
// // 			Depth:   trace.Depth,
// // 			Error:   trace.Err,
// // 		}
// // 		if trace.Stack != nil {
// // 			stack := make([]string, len(trace.Stack))
// // 			for i, stackValue := range trace.Stack {
// // 				stack[i] = stackValue.Hex()
// // 			}
// // 			formatted[index].Stack = &stack
// // 		}
// // 		if trace.Memory != nil {
// // 			memory := make([]string, 0, (len(trace.Memory)+31)/32)
// // 			for i := 0; i+32 <= len(trace.Memory); i += 32 {
// // 				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
// // 			}
// // 			formatted[index].Memory = &memory
// // 		}
// // 		if trace.Storage != nil {
// // 			storage := make(map[string]string)
// // 			for i, storageValue := range trace.Storage {
// // 				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
// // 			}
// // 			formatted[index].Storage = &storage
// // 		}
// // 	}
// // 	return formatted
// // }
//
//
// // accessListResult returns an optional accesslist
// // Its the result of the `debug_createAccessList` RPC call.
// // It contains an error if the transaction itself failed.
// type accessListResult struct {
// 	Accesslist *types.AccessList `json:"accessList"`
// 	Error      string            `json:"error,omitempty"`
// 	GasUsed    hexutil.Uint64    `json:"gasUsed"`
// }
//
// // TODO:
// // CreateAccessList creates a EIP-2930 type AccessList for the given transaction.
// // Reexec and BlockNrOrHash can be specified to create the accessList on top of a certain state.
// func (s *PublicBlockChainAPI) CreateAccessList(ctx context.Context, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) (*accessListResult, error) {
// 	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
// 	if blockNrOrHash != nil {
// 		bNrOrHash = *blockNrOrHash
// 	}
// 	acl, gasUsed, vmerr, err := AccessList(ctx, s.b, bNrOrHash, args)
// 	if err != nil {
// 		return nil, err
// 	}
// 	result := &accessListResult{Accesslist: &acl, GasUsed: hexutil.Uint64(gasUsed)}
// 	if vmerr != nil {
// 		result.Error = vmerr.Error()
// 	}
// 	return result, nil
// }
//
// // AccessList creates an access list for the given transaction.
// // If the accesslist creation fails an error is returned.
// // If the transaction itself fails, an vmErr is returned.
// func AccessList(ctx context.Context, b Backend, blockNrOrHash vm.BlockNumberOrHash, args TransactionArgs) (acl types.AccessList, gasUsed uint64, vmErr error, err error) {
// 	// Retrieve the execution context
// 	db, header, err := b.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
// 	if db == nil || err != nil {
// 		return nil, 0, nil, err
// 	}
// 	// If the gas amount is not set, extract this as it will depend on access
// 	// lists and we'll need to reestimate every time
// 	nogas := args.Gas == nil
//
// 	// Ensure any missing fields are filled, extract the recipient and input data
// 	if err := args.setDefaults(ctx, b); err != nil {
// 		return nil, 0, nil, err
// 	}
// 	var to common.Address
// 	if args.To != nil {
// 		to = *args.To
// 	} else {
// 		to = crypto.CreateAddress(args.from(), uint64(*args.Nonce))
// 	}
// 	// Retrieve the precompiles since they don't need to be added to the access list
// 	precompiles := vm.ActivePrecompiles(b.ChainConfig().Rules(header.Number))
//
// 	// Create an initial tracer
// 	prevTracer := vm.NewAccessListTracer(nil, args.from(), to, precompiles)
// 	if args.AccessList != nil {
// 		prevTracer = vm.NewAccessListTracer(*args.AccessList, args.from(), to, precompiles)
// 	}
// 	for {
// 		// Retrieve the current access list to expand
// 		accessList := prevTracer.AccessList()
// 		log.Trace("Creating access list", "input", accessList)
//
// 		// If no gas amount was specified, each unique access list needs it's own
// 		// gas calculation. This is quite expensive, but we need to be accurate
// 		// and it's convered by the sender only anyway.
// 		if nogas {
// 			args.Gas = nil
// 			if err := args.setDefaults(ctx, b); err != nil {
// 				return nil, 0, nil, err // shouldn't happen, just in case
// 			}
// 		}
// 		// Copy the original db so we don't modify it
// 		statedb := db.Copy()
// 		msg := types.NewMessage(args.from(), args.To, uint64(*args.Nonce), args.Value.ToInt(), uint64(*args.Gas), args.GasPrice.ToInt(), big.NewInt(0), big.NewInt(0), args.data(), accessList, false)
//
// 		// Apply the transaction with the access list tracer
// 		tracer := vm.NewAccessListTracer(accessList, args.from(), to, precompiles)
// 		config := vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}
// 		vmenv, _, err := b.GetEVM(ctx, msg, statedb, header, &config)
// 		if err != nil {
// 			return nil, 0, nil, err
// 		}
// 		res, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas()))
// 		if err != nil {
// 			return nil, 0, nil, fmt.Errorf("failed to apply transaction: %v err: %v", args.toTransaction().Hash(), err)
// 		}
// 		if tracer.Equal(prevTracer) {
// 			return accessList, res.UsedGas, res.Err, nil
// 		}
// 		prevTracer = tracer
// 	}
// }
//
// // PublicTransactionPoolAPI exposes methods for the RPC interface
// type PublicTransactionPoolAPI struct {
// 	b         Backend
// 	nonceLock *AddrLocker
// 	signer    types.Signer
// }
//
// // NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
// func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
// 	// The signer used by the API should always be the 'latest' known one because we expect
// 	// signers to be backwards-compatible with old transactions.
// 	signer := types.LatestSigner(b.ChainConfig())
// 	return &PublicTransactionPoolAPI{b, nonceLock, signer}
// }
//
// // TODO:
// // FillTransaction fills the defaults (nonce, gas, gasPrice or 1559 fields)
// // on a given unsigned transaction, and returns it to the caller for further
// // processing (signing + broadcast).
// func (s *PublicTransactionPoolAPI) FillTransaction(ctx context.Context, args TransactionArgs) (*SignTransactionResult, error) {
// 	// Set some sanity defaults and terminate on failure
// 	if err := args.setDefaults(ctx, s.b); err != nil {
// 		return nil, err
// 	}
// 	// Assemble the transaction and obtain rlp
// 	tx := args.toTransaction()
// 	data, err := tx.MarshalBinary()
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &SignTransactionResult{data, tx}, nil
// }
//
// // TODO: Send off to the masters via Kafka
// // SendRawTransaction will add the signed transaction to the transaction pool.
// // The sender is responsible for signing the transaction and using the correct nonce.
// func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, input hexutil.Bytes) (common.Hash, error) {
// 	tx := new(types.Transaction)
// 	if err := tx.UnmarshalBinary(input); err != nil {
// 		return common.Hash{}, err
// 	}
// 	return SubmitTransaction(ctx, s.b, tx)
// }
//
// // SignTransactionResult represents a RLP encoded signed transaction.
// type SignTransactionResult struct {
// 	Raw hexutil.Bytes      `json:"raw"`
// 	Tx  *types.Transaction `json:"tx"`
// }
//
// // PublicNetAPI offers network related RPC methods
// type PublicNetAPI struct {
// 	net            *p2p.Server
// 	networkVersion uint64
// }
//
// // NewPublicNetAPI creates a new net API instance.
// func NewPublicNetAPI(net *p2p.Server, networkVersion uint64) *PublicNetAPI {
// 	return &PublicNetAPI{net, networkVersion}
// }
//
// // TODO:
// // Listening returns an indication if the node is listening for network connections.
// func (s *PublicNetAPI) Listening() bool {
// 	return true // always listening
// }
//
// // TODO:
// // PeerCount returns the number of connected peers
// func (s *PublicNetAPI) PeerCount() hexutil.Uint {
// 	return hexutil.Uint(s.net.PeerCount())
// }
//
// // TODO:
// // Version returns the current ethereum protocol version.
// func (s *PublicNetAPI) Version() string {
// 	return fmt.Sprintf("%d", s.networkVersion)
// }
