package api

import (
	"math/big"
	"time"
	"context"
	"fmt"
	"errors"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	rpc "github.com/openrelayxyz/cardinal-rpc"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	ptypes "github.com/openrelayxyz/plugeth-utils/restricted/types"
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

type simBlockResult struct {
	fullTx      bool
	chainConfig *params.ChainConfig
	Block       *ptypes.Block
	Calls       []simCallResult
	// senders is a map of transaction hashes to their senders.
	senders map[ctypes.Hash]common.Address
}

// simCallResult is the result of a simulated call.
type simCallResult struct {
	ReturnValue hexutil.Bytes  `json:"returnData"`
	Logs        []*types.Log   `json:"logs"`
	GasUsed     hexutil.Uint64 `json:"gasUsed"`
	Status      hexutil.Uint64 `json:"status"`
	Error       *callError     `json:"error,omitempty"`
}

func (s *simulator) execute(ctx *rpc.CallContext, blocks []simBlock) ([]*simBlockResult, error){
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
	
	santizedblocks, err := s.sanitizeChain(blocks)
	if err != nil {
		return nil, err
	}

	// yet to implement
	headers, err := s.makeHeaders(santizedblocks)
	if err != nil {
		return nil, err
	}

}

// sanitizeChain checks the chain integrity. Specifically it checks that
// block numbers and timestamp are strictly increasing, setting default values
// when necessary. Gaps in block numbers are filled with empty blocks.
func (s *simulator) sanitizeChain(blocks []simBlock) ([]simBlock, error) {
	var (
		res           = make([]simBlock, 0, len(blocks))
		base          = s.base
		prevNumber    = base.Number
		prevTimestamp = base.Time
	)

	for _, block := range blocks {
		if block.BlockOverrides == nil {
			block.BlockOverrides = &BlockOverrides{}
		}

		if block.BlockOverrides.Number == nil {
			n := new(big.Int).Add(prevNumber, big.NewInt(1))
			block.BlockOverrides.Number = (*hexutil.Big)(n)
		}

		diff := new(big.Int).Sub(block.BlockOverrides.Number.ToInt(), prevNumber)
		if diff.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("block numbers must be in order: %d <= %d", 
				block.BlockOverrides.Number.ToInt().Uint64(), prevNumber.Uint64())
		}

		if total := new(big.Int).Sub(block.BlockOverrides.Number.ToInt(), base.Number); total.Cmp(big.NewInt(maxSimulateBlocks)) > 0 {
			return nil, fmt.Errorf("too many blocks")
		}

		// Fill gaps with empty blocks
		if diff.Cmp(big.NewInt(1)) > 0 {
			gap := new(big.Int).Sub(diff, big.NewInt(1))
			for i := uint64(0); i < gap.Uint64(); i++ {
				n := new(big.Int).Add(prevNumber, big.NewInt(int64(i+1)))
				t := prevTimestamp + timestampIncrement
				emptyBlock := simBlock{
					BlockOverrides: &BlockOverrides{
						Number: (*hexutil.Big)(n),
						Time:   (*hexutil.Uint64)(&t),
					},
				}
				prevTimestamp = t
				res = append(res, emptyBlock)
			}
		}

		prevNumber = block.BlockOverrides.Number.ToInt()
		var t uint64
		if block.BlockOverrides.Time == nil {
			t = prevTimestamp + timestampIncrement
			block.BlockOverrides.Time = (*hexutil.Uint64)(&t)
		} else {
			t = uint64(*block.BlockOverrides.Time)
			if t <= prevTimestamp {
				return nil, fmt.Errorf("block timestamps must be in order: %d <= %d", t, prevTimestamp)
			}
		}
		prevTimestamp = t
		res = append(res, block)
	}

	return res, nil
}

