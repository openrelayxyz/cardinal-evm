package tracers

import (
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/state"
)

// TracerResult implements the tracer methods of a VM tracer, but also returns
// a Result
type TracerResult interface {
	vm.Tracer
	Result() interface{}
}

// tracerRegistry tracks TracerResult implementations available for debug_trace
// methods
type tracerRegistry map[string]func (state.StateDB, *vm.LogConfig) TracerResult

var (
	defaultTracerRegistry tracerRegistry
)

// Register adds a new tracer to the default registry
func Register(name string, tracer func (state.StateDB, *vm.LogConfig) TracerResult) {
	defaultTracerRegistry[name] = tracer
}

// Get retrieves a tracer from the default registry
func Get(name string, statedb state.StateDB, cfg *vm.LogConfig) (TracerResult, bool) {
	if fn, ok := defaultTracerRegistry[name]; ok {
		return fn(statedb, cfg), true
	}
	return nil, false
}
