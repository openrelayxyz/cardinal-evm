package tracers

import (
	"fmt"
	"time"
	"math/big"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
)


// executionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type executionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []structLogRes `json:"structLogs"`
}

// structLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type structLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   string             `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// FormatLogs formats EVM returned structured logs for json output
func formatLogs(logs []vm.StructLog) []structLogRes {
	formatted := make([]structLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = structLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.ErrorString(),
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = stackValue.Hex()
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}


type StructLogger struct {
	sl *vm.StructLogger
	gas uint64
	output []byte
	err error
}

func (s *StructLogger) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	s.sl.CaptureStart(from, to, create, input, gas, value)
}
func (s *StructLogger) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	s.sl.CaptureState(pc, op, gas, cost, scope, rData, depth, err)
}
func (s *StructLogger) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}
func (s *StructLogger) CaptureExit(output []byte, gasUsed uint64, err error) {
}
func (s *StructLogger) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
	s.sl.CaptureFault(pc, op, gas, cost, scope, depth, err)
}
func (s *StructLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
	s.sl.CaptureEnd(output, gasUsed, t, err)
	s.output = output
	s.gas = gasUsed
	s.err = err
}
func (s *StructLogger) Result() interface{} {
	return &executionResult{
		Gas:         s.gas,
		Failed:      s.err != nil,
		ReturnValue: string(s.output),
		StructLogs:  formatLogs(s.sl.StructLogs()),
	}
}

func init() {
	Register("default", func(sdb state.StateDB, cfg *vm.LogConfig) TracerResult {
		return &StructLogger{
			sl: vm.NewStructLogger(cfg, sdb),
		}
	})
}
