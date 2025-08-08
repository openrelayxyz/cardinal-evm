// Copyright 2024 The go-ethereum Authors
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

package tracers

import (
	"encoding/json"
	"math/big"

	"github.com/openrelayxyz/cardinal-types"
	// "github.com/ethereum/go-ethereum/core/tracing"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/holiman/uint256"
)

// Context contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type Context struct {
	BlockHash   types.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int    // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int         // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      types.Hash // Hash of the transaction being traced (zero if dangling call)
}

// Tracer represents the set of methods that must be exposed by a tracer
// for it to be available through the RPC interface.
// This involves a method to retrieve results and one to
// stop tracing.
type Tracer struct {
	*tracing.Hooks
	GetResult func() (json.RawMessage, error)
	// Stop terminates execution of the tracer at the first opportune moment.
	Stop func(err error)
}

type ctorFn func(*Context, json.RawMessage, *params.ChainConfig) (*Tracer, error)
type jsCtorFn func(string, *Context, json.RawMessage, *params.ChainConfig) (*Tracer, error)

type elem struct {
	ctor ctorFn
	isJS bool
}

// DefaultDirectory is the collection of tracers bundled by default.
var DefaultDirectory = directory{elems: make(map[string]elem)}

// directory provides functionality to lookup a tracer by name
// and a function to instantiate it. It falls back to a JS code evaluator
// if no tracer of the given name exists.
type directory struct {
	elems  map[string]elem
	jsEval jsCtorFn
}

// Register registers a method as a lookup for tracers, meaning that
// users can invoke a named tracer through that lookup.
func (d *directory) Register(name string, f ctorFn, isJS bool) {
	d.elems[name] = elem{ctor: f, isJS: isJS}
}

// RegisterJSEval registers a tracer that is able to parse
// dynamic user-provided JS code.
func (d *directory) RegisterJSEval(f jsCtorFn) {
	d.jsEval = f
}

// New returns a new instance of a tracer, by iterating through the
// registered lookups. Name is either name of an existing tracer
// or an arbitrary JS code.
func (d *directory) New(name string, ctx *Context, cfg json.RawMessage, chainConfig *params.ChainConfig) (*Tracer, error) {
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	if elem, ok := d.elems[name]; ok {
		return elem.ctor(ctx, cfg, chainConfig)
	}
	// Assume JS code
	return d.jsEval(name, ctx, cfg, chainConfig)
}

// IsJS will return true if the given tracer will evaluate
// JS code. Because code evaluation has high overhead, this
// info will be used in determining fast and slow code paths.
func (d *directory) IsJS(name string) bool {
	if elem, ok := d.elems[name]; ok {
		return elem.isJS
	}
	// JS eval will execute JS code
	return true
}

type (
	/*
		- VM events -
	*/

	// TxStartHook is called before the execution of a transaction starts.
	// Call simulations don't come with a valid signature. `from` field
	// to be used for address of the caller.
	TxStartHook = func(vm *VMContext, tx *types.Transaction, from common.Address)

	// TxEndHook is called after the execution of a transaction ends.
	TxEndHook = func(receipt *types.Receipt, err error)

	// EnterHook is invoked when the processing of a message starts.
	//
	// Take note that EnterHook, when in the context of a live tracer, can be invoked
	// outside of the `OnTxStart` and `OnTxEnd` hooks when dealing with system calls,
	// see [OnSystemCallStartHook] and [OnSystemCallEndHook] for more information.
	EnterHook = func(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int)

	// ExitHook is invoked when the processing of a message ends.
	// `revert` is true when there was an error during the execution.
	// Exceptionally, before the homestead hardfork a contract creation that
	// ran out of gas when attempting to persist the code to database did not
	// count as a call failure and did not cause a revert of the call. This will
	// be indicated by `reverted == false` and `err == ErrCodeStoreOutOfGas`.
	//
	// Take note that ExitHook, when in the context of a live tracer, can be invoked
	// outside of the `OnTxStart` and `OnTxEnd` hooks when dealing with system calls,
	// see [OnSystemCallStartHook] and [OnSystemCallEndHook] for more information.
	ExitHook = func(depth int, output []byte, gasUsed uint64, err error, reverted bool)

	// OpcodeHook is invoked just prior to the execution of an opcode.
	OpcodeHook = func(pc uint64, op byte, gas, cost uint64, scope OpContext, rData []byte, depth int, err error)

	// FaultHook is invoked when an error occurs during the execution of an opcode.
	FaultHook = func(pc uint64, op byte, gas, cost uint64, scope OpContext, depth int, err error)

	// GasChangeHook is invoked when the gas changes.
	GasChangeHook = func(old, new uint64, reason GasChangeReason)

	/*
		- Chain events -
	*/

	// BlockchainInitHook is called when the blockchain is initialized.
	BlockchainInitHook = func(chainConfig *params.ChainConfig)

	// CloseHook is called when the blockchain closes.
	CloseHook = func()

	// BlockStartHook is called before executing `block`.
	// `td` is the total difficulty prior to `block`.
	BlockStartHook = func(event BlockEvent)

	// BlockEndHook is called after executing a block.
	BlockEndHook = func(err error)

	// SkippedBlockHook indicates a block was skipped during processing
	// due to it being known previously. This can happen e.g. when recovering
	// from a crash.
	SkippedBlockHook = func(event BlockEvent)

	// GenesisBlockHook is called when the genesis block is being processed.
	GenesisBlockHook = func(genesis *types.Block, alloc types.GenesisAlloc)

	// OnSystemCallStartHook is called when a system call is about to be executed. Today,
	// this hook is invoked when the EIP-4788 system call is about to be executed to set the
	// beacon block root.
	//
	// After this hook, the EVM call tracing will happened as usual so you will receive a `OnEnter/OnExit`
	// as well as state hooks between this hook and the `OnSystemCallEndHook`.
	//
	// Note that system call happens outside normal transaction execution, so the `OnTxStart/OnTxEnd` hooks
	// will not be invoked.
	OnSystemCallStartHook = func()

	// OnSystemCallStartHookV2 is called when a system call is about to be executed. Refer
	// to `OnSystemCallStartHook` for more information.
	OnSystemCallStartHookV2 = func(vm *VMContext)

	// OnSystemCallEndHook is called when a system call has finished executing. Today,
	// this hook is invoked when the EIP-4788 system call is about to be executed to set the
	// beacon block root.
	OnSystemCallEndHook = func()

	/*
		- State events -
	*/

	// BalanceChangeHook is called when the balance of an account changes.
	BalanceChangeHook = func(addr common.Address, prev, new *big.Int, reason BalanceChangeReason)

	// NonceChangeHook is called when the nonce of an account changes.
	NonceChangeHook = func(addr common.Address, prev, new uint64)

	// NonceChangeHookV2 is called when the nonce of an account changes.
	NonceChangeHookV2 = func(addr common.Address, prev, new uint64, reason NonceChangeReason)

	// CodeChangeHook is called when the code of an account changes.
	CodeChangeHook = func(addr common.Address, prevCodeHash common.Hash, prevCode []byte, codeHash common.Hash, code []byte)

	// StorageChangeHook is called when the storage of an account changes.
	StorageChangeHook = func(addr common.Address, slot common.Hash, prev, new common.Hash)

	// LogHook is called when a log is emitted.
	LogHook = func(log *types.Log)

	// BlockHashReadHook is called when EVM reads the blockhash of a block.
	BlockHashReadHook = func(blockNumber uint64, hash common.Hash)
)

type Hooks struct {
	// VM events
	OnTxStart   TxStartHook
	OnTxEnd     TxEndHook
	OnEnter     EnterHook
	OnExit      ExitHook
	OnOpcode    OpcodeHook
	OnFault     FaultHook
	OnGasChange GasChangeHook
	// Chain events
	OnBlockchainInit    BlockchainInitHook
	OnClose             CloseHook
	OnBlockStart        BlockStartHook
	OnBlockEnd          BlockEndHook
	OnSkippedBlock      SkippedBlockHook
	OnGenesisBlock      GenesisBlockHook
	OnSystemCallStart   OnSystemCallStartHook
	OnSystemCallStartV2 OnSystemCallStartHookV2
	OnSystemCallEnd     OnSystemCallEndHook
	// State events
	OnBalanceChange BalanceChangeHook
	OnNonceChange   NonceChangeHook
	OnNonceChangeV2 NonceChangeHookV2
	OnCodeChange    CodeChangeHook
	OnStorageChange StorageChangeHook
	OnLog           LogHook
	// Block hash read
	OnBlockHashRead BlockHashReadHook
}

// OpContext provides the context at which the opcode is being
// executed in, including the memory, stack and various contract-level information.
type OpContext interface {
	MemoryData() []byte
	StackData() []uint256.Int
	Caller() common.Address
	Address() common.Address
	CallValue() *uint256.Int
	CallInput() []byte
	ContractCode() []byte
}
