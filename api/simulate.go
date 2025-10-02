package api

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"
	"encoding/json"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/eips/eip1559"
	"github.com/openrelayxyz/cardinal-evm/eips/eip4844"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	rpc "github.com/openrelayxyz/cardinal-rpc"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

const (
	// maxSimulateBlocks is the maximum number of blocks that can be simulated
	// in a single request.
	maxSimulateBlocks = 256

	// timestampIncrement is the default increment between block timestamps.
	timestampIncrement = 1
)

// BlockOverrides is a set of header fields to override.
type BlockOverrides struct {
	Number        *hexutil.Big
	Difficulty    *hexutil.Big // No-op if we're simulating post-merge calls.
	Time          *hexutil.Uint64
	GasLimit      *hexutil.Uint64
	FeeRecipient  *common.Address
	PrevRandao    *ctypes.Hash
	BaseFeePerGas *hexutil.Big
	BlobBaseFee   *hexutil.Big
	BeaconRoot    *ctypes.Hash
	Withdrawals   *types.Withdrawals
}

// simOpts are the inputs to eth_simulateV1.
type simOpts struct {
	BlockStateCalls        []simBlock
	TraceTransfers         bool
	Validation             bool
	ReturnFullTransactions bool
}

// simBlock is a batch of calls to be simulated sequentially.
type simBlock struct {
	BlockOverrides *BlockOverrides
	StateOverrides *StateOverride
	Calls          []TransactionArgs
}

// simulator is a stateful object that simulates a series of blocks.
// it is not safe for concurrent use.
type simulator struct {
	timeout        time.Duration
	state          state.StateDB
	base           *types.Header
	chainConfig    *params.ChainConfig
	gp             *GasPool
	traceTransfers bool
	validate       bool
	fullTx         bool
	evmFn          func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM
}

// simCallResult is the result of a simulated call.
type simCallResult struct {
	ReturnValue hexutil.Bytes  `json:"returnData"`
	Logs        []*types.Log   `json:"logs"`
	GasUsed     hexutil.Uint64 `json:"gasUsed"`
	Status      hexutil.Uint64 `json:"status"`
	Error       *callError     `json:"error,omitempty"`
}

func (r *simCallResult) MarshalJSON() ([]byte, error) {
	type callResultAlias simCallResult
	// Marshal logs to be an empty array instead of nil when empty
	if r.Logs == nil {
		r.Logs = []*types.Log{}
	}
	return json.Marshal((*callResultAlias)(r))
}

type simBlockResult struct {
	fullTx      bool
	chainConfig *params.ChainConfig
	Block       *types.Block
	Calls       []simCallResult
	// senders is a map of transaction hashes to their senders.
	senders map[ctypes.Hash]common.Address
}

func (r *simBlockResult) MarshalJSON() ([]byte, error) {
	blockData := types.RPCMarshalBlock(r.Block, true, r.fullTx, r.chainConfig)
	blockData["calls"] = r.Calls
	// Set tx sender if user requested full tx objects.
	if r.fullTx {
		if raw, ok := blockData["transactions"].([]any); ok {
			for _, tx := range raw {
				if tx, ok := tx.(*types.RPCTransaction); ok {
					tx.From = r.senders[tx.Hash]
				} else {
					return nil, errors.New("simulated transaction result has invalid type")
				}
			}
		}
	}
	return json.Marshal(blockData)
}

type simpleTrieHasher struct {
	data []byte
}

func (h *simpleTrieHasher) Reset() {
	h.data = h.data[:0] 
}

func (h *simpleTrieHasher) Update(key, value []byte) {
	h.data = append(h.data, key...)
	h.data = append(h.data, value...)
}

func (h *simpleTrieHasher) Hash() ctypes.Hash {
	return crypto.Keccak256Hash(h.data)
}

func (s *simulator) execute(ctx *rpc.CallContext, blocks []simBlock) ([]*simBlockResult, error) {
	if err := ctx.Context().Err(); err != nil {
		return nil, err
	}

	var cancel context.CancelFunc
	var execCtx context.Context
	if s.timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx.Context(), s.timeout)
	} else {
		execCtx, cancel = context.WithCancel(ctx.Context())
	}
	defer cancel()

	var (
		results = make([]*simBlockResult, len(blocks))
		headers = make([]*types.Header, 0, len(blocks))
		parent  = s.base
	)

	var err error
	blocks, err = s.sanitizeChain(blocks)
	if err != nil {
		return nil, err
	}

	for bi, block := range blocks {
		header := types.CopyHeader(s.base)
		header.Number = new(big.Int).Add(s.base.Number, big.NewInt(int64(bi+1)))
		header.ParentHash = parent.Hash()
		header.Time = parent.Time + uint64(bi+1)*12

		override := *block.BlockOverrides
		if override.Number != nil {header.Number = override.Number.ToInt()}
		if override.Difficulty != nil {header.Difficulty = override.Difficulty.ToInt()}
		if override.Time != nil {header.Time = uint64(*override.Time)}
		if override.GasLimit != nil {header.GasLimit = uint64(*override.GasLimit)}
		if override.FeeRecipient != nil {header.Coinbase = *override.FeeRecipient}
		if override.PrevRandao != nil {header.MixDigest = *override.PrevRandao}
		if override.BaseFeePerGas != nil {header.BaseFee = override.BaseFeePerGas.ToInt()}
		if override.BlobBaseFee != nil {
			val := *override.BlobBaseFee.ToInt()
			ptr := val.Uint64()
			header.ExcessBlobGas = &ptr
		}

		s.gp = new(GasPool).AddGas(header.GasLimit)

		if err := execCtx.Err(); err != nil {
			return nil, err
		}
		result, callResults, senders, err := s.processBlock(ctx, &block, header, parent, headers, s.timeout)
		if err != nil {
			return nil, err
		}
		results[bi] = &simBlockResult{fullTx: s.fullTx, chainConfig: s.chainConfig, Block: result, Calls: callResults, senders: senders}
		headers = append(headers, result.Header())
		parent = result.Header()

		s.state.Finalise()
	}

	return results, nil
}

func (s *simulator) processBlock(ctx *rpc.CallContext, block *simBlock, header, parent *types.Header, headers []*types.Header, timeout time.Duration) (*types.Block, []simCallResult, map[ctypes.Hash]common.Address, error) {
	// Set header fields that depend only on parent block.
	// Parent hash is needed for evm.GetHashFn to work.
	header.ParentHash = parent.Hash()
	if s.chainConfig.IsLondon(header.Number) {
		if header.BaseFee == nil {
			if s.validate {
				header.BaseFee = eip1559.CalcBaseFee(s.chainConfig, parent)
			} else {
				header.BaseFee = big.NewInt(0)
			}
		}
	}
	if s.chainConfig.IsCancun(header.Number, new(big.Int).SetUint64(header.Time)) {
		var excess uint64
		if s.chainConfig.IsCancun(parent.Number, new(big.Int).SetUint64(parent.Time)) {
			parentExcess := uint64(0)
			if parent.ExcessBlobGas != nil {
				parentExcess = *parent.ExcessBlobGas
			}
			parentBlobGasUsed := uint64(0)
			if parent.BlobGasUsed != nil {
				parentBlobGasUsed = *parent.BlobGasUsed
			}
			excess = eip4844.CalcExcessBlobGas(parentExcess, parentBlobGasUsed)
		}
		header.ExcessBlobGas = &excess
	}
	
	// State overrides are applied prior to execution of a block
	if block.StateOverrides != nil {
		if err := block.StateOverrides.Apply(s.state); err != nil {
			return nil, nil, nil, err
		}
	}

	var (
		blobGasUsed uint64
		gasUsed     uint64
		txes        = make([]*types.Transaction, len(block.Calls))
		callResults = make([]simCallResult, len(block.Calls))
		receipts    = make([]*types.Receipt, len(block.Calls))
		senders     = make(map[ctypes.Hash]common.Address)
	)

	getHashFn := func(n uint64) ctypes.Hash {
		for _, h := range headers {
			if h.Number.Uint64() == n {
				return h.Hash()
			}
		}
		if parent.Number.Uint64() == n {
			return parent.Hash()
		}
		return ctypes.Hash{}
	}

	for i, call := range block.Calls {
		tracer := newTracer(s.traceTransfers, header.Number.Uint64(), header.Hash(), ctypes.Hash{}, uint(i))
		evm := s.evmFn(s.state, &vm.Config{
			NoBaseFee: !s.validate, Tracer: tracer,
		}, call.from(), call.GasPrice.ToInt())

		if err := ctx.Context().Err(); err != nil {
			return nil, nil, nil, err
		}
		if err := call.setDefaults(ctx, s.evmFn, s.state, header, vm.BlockNumberOrHashWithHash(header.Hash(), false)); err != nil {
			return nil, nil, nil, err
		}
		if gasUsed+uint64(*call.Gas) > header.GasLimit {
			return nil,nil,nil, &blockGasLimitReachedError{fmt.Sprintf("block gas limit reached: %d >= %d", gasUsed, header.GasLimit)}
		}
		if call.Nonce == nil {
			nonce := evm.StateDB.GetNonce(call.from())
			call.Nonce = (*hexutil.Uint64)(&nonce)
		}
		// Let the call run wild unless explicitly specified.
		if call.Gas == nil {
			remaining := header.GasLimit - gasUsed
        	call.Gas = (*hexutil.Uint64)(&remaining)
		}
		tx := call.ToTransaction(types.DynamicFeeTxType)
		txes[i] = tx
		senders[tx.Hash()] = call.from()
		tracer.reset(tx.Hash(), uint(i))
		s.state.SetTxContext(tx.Hash(), i)

		evm.Context.BaseFee = header.BaseFee

		if evm.Context.GetHash == nil {
			evm.Context.GetHash = getHashFn
		}

		msg, err := call.ToMessage(s.gp.Gas(), header.BaseFee)
		if err != nil {
			return nil, nil, nil, err
		}
		result, err := applyMessageWithEVM(ctx, evm, &msg, timeout, s.gp)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("transaction execution failed: %v", err)
		}

		var root []byte
		if s.chainConfig.IsByzantium(header.Number) {
			s.state.Finalise()
		} else {
			root = nil
		}
		gasUsed += result.UsedGas
		header.Root = s.base.Root

		receipt := &types.Receipt{
			Type:              tx.Type(),
			PostState:         root,
			Status:            types.ReceiptStatusSuccessful,
			CumulativeGasUsed: gasUsed,
			TxHash:            tx.Hash(),
			GasUsed:           result.UsedGas,
			TransactionIndex:  uint(i),
		}
		receipt.Logs = s.state.GetLogs(tx.Hash(), header.Number.Uint64(), header.Hash())
		if s.traceTransfers {
			receipt.Logs = append(receipt.Logs, tracer.Logs()...)
		}
		receipt.Bloom = types.CreateBloom([]*types.Receipt{receipt})
		if tx.To() == nil {
			receipt.ContractAddress = crypto.CreateAddress(*call.From, tx.Nonce())
		}
		if result.Failed() {
			receipt.Status = types.ReceiptStatusFailed
		}

		// Handle blob gas for Cancun
		if s.chainConfig.IsCancun(header.Number, new(big.Int).SetUint64(header.Time)) {
			if tx.Type() == types.BlobTxType {
				receipt.BlobGasUsed = tx.BlobGas()
				blobGasUsed += receipt.BlobGasUsed
			}
		}

		receipts[i] = receipt

		callRes := simCallResult{
			GasUsed:  hexutil.Uint64(result.UsedGas),
			ReturnValue: result.ReturnData,
			Logs:  receipt.Logs,
		}
		if result.Failed() {
			callRes.Status = hexutil.Uint64(types.ReceiptStatusFailed)
			if errors.Is(result.Err, vm.ErrExecutionReverted) {
				// If the result contains a revert reason, try to unpack it.
				revertErr := newRevertError(result)
				callRes.Error = &callError{Message: revertErr.Error(), Code: errCodeReverted, Data: revertErr.ErrorData().(string)}
			} else {
				callRes.Error = &callError{Message: result.Err.Error(), Code: errCodeVMError}
			}
		} else {
			callRes.Status = hexutil.Uint64(types.ReceiptStatusSuccessful)
		}
		callResults[i] = callRes
	}

	header.GasUsed = gasUsed
	if s.chainConfig.IsCancun(header.Number, new(big.Int).SetUint64(header.Time)) {
		header.BlobGasUsed = &blobGasUsed
	}

	hasher := &simpleTrieHasher{}

	header.TxHash = types.DeriveSha(types.Transactions(txes), hasher)
	header.ReceiptHash = types.DeriveSha(types.Receipts(receipts), hasher)
	header.Bloom = types.CreateBloom(receipts)
	header.Root = s.base.Root

	blockBody := &types.Body{Transactions: txes, Withdrawals: *block.BlockOverrides.Withdrawals}
	blck := types.NewBlock(header, blockBody, receipts, hasher)

	return blck, callResults, senders, nil
}

// sanitizeChain checks the chain integrity. Specifically it checks that
// block numbers and timestamp are strictly increasing, setting default values
// when necessary. Gaps in block numbers are filled with empty blocks.
// Note: It modifies the block's override object.
func (s *simulator) sanitizeChain(blocks []simBlock) ([]simBlock, error) {
	var (
		res           = make([]simBlock, 0, len(blocks))
		base          = s.base
		prevNumber    = base.Number
		prevTimestamp = base.Time
	)
	for _, block := range blocks {
		if block.BlockOverrides == nil {
			block.BlockOverrides = new(BlockOverrides)
		}
		if block.BlockOverrides.Number == nil {
			n := new(big.Int).Add(prevNumber, big.NewInt(1))
			block.BlockOverrides.Number = (*hexutil.Big)(n)
		}
		if block.BlockOverrides.Withdrawals == nil {
			block.BlockOverrides.Withdrawals = &types.Withdrawals{}
		}
		diff := new(big.Int).Sub(block.BlockOverrides.Number.ToInt(), prevNumber)
		if diff.Cmp(common.Big0) <= 0 {
			return nil, &invalidBlockNumberError{fmt.Sprintf("block numbers must be in order: %d <= %d", block.BlockOverrides.Number.ToInt().Uint64(), prevNumber)}
		}
		if total := new(big.Int).Sub(block.BlockOverrides.Number.ToInt(), base.Number); total.Cmp(big.NewInt(maxSimulateBlocks)) > 0 {
			return nil, &clientLimitExceededError{message: "too many blocks"}
		}
		if diff.Cmp(big.NewInt(1)) > 0 {
			// Fill the gap with empty blocks.
			gap := new(big.Int).Sub(diff, big.NewInt(1))
			// Assign block number to the empty blocks.
			for i := uint64(0); i < gap.Uint64(); i++ {
				n := new(big.Int).Add(prevNumber, big.NewInt(int64(i+1)))
				t := prevTimestamp + timestampIncrement
				b := simBlock{
					BlockOverrides: &BlockOverrides{
						Number:      (*hexutil.Big)(n),
						Time:        (*hexutil.Uint64)(&t),
						Withdrawals: &types.Withdrawals{},
					},
				}
				prevTimestamp = t
				res = append(res, b)
			}
		}
		// Only append block after filling a potential gap.
		prevNumber = block.BlockOverrides.Number.ToInt()
		var t uint64
		if block.BlockOverrides.Time == nil {
			t = prevTimestamp + timestampIncrement
			block.BlockOverrides.Time = (*hexutil.Uint64)(&t)
		} else {
			t = uint64(*block.BlockOverrides.Time)
			if t <= prevTimestamp {
				return nil, &invalidBlockTimestampError{fmt.Sprintf("block timestamps must be in order: %d <= %d", t, prevTimestamp)}
			}
		}
		prevTimestamp = t
		res = append(res, block)
	}
	return res, nil
}

// there is a virtual log being returned by geth on any eth transfer. It is present in geth and we need to figure out how to implement it in EVM.
// aparently geth removes the extra data field from the block which we ran the call against, while evm includes it.
// look into how the block hash is being produced -- Probably not worth trying to do.  
// evm includes mix hash where geth does not. 
// look into parent beacon block root and why it appears to be left off from geth. 
// receipt root should be achievable (maybe geth is including the virtual log in the receipts)
// geth returns a size value where evm appears not to. 
// research why evm is producing a different tx hash
// evm is not including withdrawals and withdrawals hash in what we return but should. 
