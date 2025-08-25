package tracers

import (
	"encoding/json"
	"math/big"
	"time"
	"strconv"
	"sync/atomic"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
)

func init() {
	Register("4byteTracer", newFourByteTracer)
}


// fourByteTracer searches for 4byte-identifiers, and collects them for post-processing.
// It collects the methods identifiers along with the size of the supplied data, so
// a reversed signature can be matched against the size of the data.
//
// Example:
//
//	> debug.traceTransaction( "0x214e597e35da083692f5386141e69f47e973b2c56e7a8073b1ea08fd7571e9de", {tracer: "4byteTracer"})
//	{
//	  0x27dc297e-128: 1,
//	  0x38cc4831-0: 2,
//	  0x524f3889-96: 1,
//	  0xadf59f99-288: 1,
//	  0xc281d19e-0: 1
//	}
type fourByteTracer struct {
	ids               map[string]int // ids aggregates the 4byte ids found
	interrupt         atomic.Bool    // Atomic flag to signal execution interruption
	reason            error          // Textual reason for the interruption
	chainConfig       *params.ChainConfig
	activePrecompiles []common.Address // Updated on tx start based on given rules
}

// newFourByteTracer returns a native go tracer which collects
// 4 byte-identifiers of a tx, and implements vm.EVMLogger.
func newFourByteTracer(cfg json.RawMessage, chainConfig *params.ChainConfig) (vm.Tracer, error) {
	t := &fourByteTracer{
		ids:         make(map[string]int),
	}
	return t, nil
}

// isPrecompiled returns whether the addr is a precompile. Logic borrowed from newJsTracer in eth/tracers/js/tracer.go
func (t *fourByteTracer) isPrecompiled(addr common.Address) bool {
	for _, p := range t.activePrecompiles {
		if p == addr {
			return true
		}
	}
	return false
}

// store saves the given identifier and datasize.
func (t *fourByteTracer) store(id []byte, size int) {
	key := bytesToHex(id) + "-" + strconv.Itoa(size)
	t.ids[key] += 1
}


func (t *fourByteTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
    // Skip if tracing was interrupted
    if t.interrupt.Load() {
        return
    }
    if len(input) < 4 {
        return
    }
    
    if typ != vm.DELEGATECALL && typ != vm.STATICCALL &&
        typ != vm.CALL && typ != vm.CALLCODE {
        return
    }
    
    // Skip any pre-compile invocations
    if t.isPrecompiled(to) {
        return
    }
    t.store(input[0:4], len(input)-4)
}

func (t *fourByteTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {}
func (t *fourByteTracer) CaptureEnd([]byte, uint64, time.Duration, error) { }
func (t *fourByteTracer) CaptureExit(output []byte, gasUsed uint64, err error) {}
func (t *fourByteTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {}
func (t *fourByteTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error){}
func (t *fourByteTracer) CaptureLog(log *types.Log){}

// GetResult returns the json-encoded nested list of call traces, and any
// error arising from the encoding or forceful termination (via `Stop`).
func (t *fourByteTracer) GetResult() (json.RawMessage, error) {
	res, err := json.Marshal(t.ids)
	if err != nil {
		return nil, err
	}
	return res, t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *fourByteTracer) Stop(err error) {
	t.reason = err
	t.interrupt.Store(true)
}

func bytesToHex(s []byte) string {
	return "0x" + common.Bytes2Hex(s)
}
