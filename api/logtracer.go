package api

import (
	"fmt"
	"math/big"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	ctypes "github.com/openrelayxyz/cardinal-types"
)

var (
	// keccak256("Transfer(address,address,uint256)")
	transferTopic = ctypes.HexToHash("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	// ERC-7528
	transferAddress = common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE")
)


type tracer struct {
	// logs keeps logs for all open call frames.
	// This lets us clear logs for failed calls.
	logs           []*types.Log
	count          int
	traceTransfers bool
	blockNumber    uint64
	blockHash      ctypes.Hash
	txHash         ctypes.Hash
	txIdx          uint
}

func newTracer(traceTransfers bool, blockNumber uint64, blockHash, txHash ctypes.Hash, txIndex uint) *tracer {
	log.Error(fmt.Sprintf("creating tracer traceTransfers: %v, txHash: %s", traceTransfers, txHash.Hex()))
	return &tracer{
		logs:           make([]*types.Log, 0),
		traceTransfers: traceTransfers,
		blockNumber:    blockNumber,
		blockHash:      blockHash,
		txHash:         txHash,
		txIdx:          txIndex,
	}
}

func (t *tracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	log.Error("inside captureenter")
	if !t.traceTransfers {
		return
	}
	if typ != vm.DELEGATECALL && value != nil && value.Sign() > 0 {
		topics := []ctypes.Hash{
			transferTopic,
			ctypes.BytesToHash(from.Bytes()),
			ctypes.BytesToHash(to.Bytes()),
		}
		t.logs = append(t.logs, &types.Log{
			Address:     transferAddress,
			Topics:      topics,
			Data:        ctypes.BigToHash(value).Bytes(),
			BlockNumber: t.blockNumber,
			BlockHash:   t.blockHash,
			TxHash:      t.txHash,
			TxIndex:     t.txIdx,
			Index:       uint(t.count),
		})
		t.count++
	}
}

func (t *tracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int){
	log.Error("inside capturestart")
}
func (t *tracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error){}
func (t *tracer) CaptureExit(output []byte, gasUsed uint64, err error){}
func (t *tracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error){}
func (t *tracer) CaptureEnd(output []byte, gasUsed uint64, time time.Duration, err error){}

// reset prepares the tracer for the next transaction.
func (t *tracer) reset(txHash ctypes.Hash, txIdx uint) {
	t.logs = make([]*types.Log, 0)
	t.count = 0 
	t.txHash = txHash
	t.txIdx = txIdx
}

func (t *tracer) Logs() []*types.Log {
	return t.logs
}