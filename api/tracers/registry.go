package tracers

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-types"
)

// Context contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type Context struct {
	BlockHash   types.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int    // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int         // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      types.Hash // Hash of the transaction being traced (zero if dangling call)
}

type tracerInfo struct {
    factory TracerFactory
    isJS    bool
}

type TracerFactory func(ctx *Context, config json.RawMessage) (vm.Tracer, error)
var jsEvalFactory func(string, *Context, json.RawMessage) (vm.Tracer, error)
var Registry = make(map[string]tracerInfo)

func Register(name string, factory TracerFactory, isJS bool) {
    Registry[name] = tracerInfo{
		factory: factory,
		isJS: isJS,
	}
}

func RegisterJSEval(f func(string, *Context, json.RawMessage) (vm.Tracer, error)) {
    jsEvalFactory = f
}


func New(name string, ctx *Context, config json.RawMessage) (vm.Tracer, error) {
    if info, exists := Registry[name]; exists {
        return info.factory(ctx, config)
    }
    // Fallback to JS eval for unknown names
    if jsEvalFactory != nil {
        return jsEvalFactory(name, ctx, config)
    }
    return nil, fmt.Errorf("unknown tracer: %s", name)
}