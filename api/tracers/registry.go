package tracers

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/openrelayxyz/cardinal-evm/params"
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

type StateInjector interface {
    SetVMContext(ctx *VMContext)
}

type TracerFactory func(ctx *Context, config json.RawMessage, chainConfig *params.ChainConfig) (vm.Tracer, error)

type tracerInfo struct {
	factory TracerFactory
	isJS  bool
}

var Registry = make(map[string]tracerInfo)

func Register(name string, factory TracerFactory, isJS bool) {
    Registry[name] = tracerInfo{
		factory: factory,
		isJS: isJS,
	}
}

func New(name string, ctx *Context, config json.RawMessage, chainConfig *params.ChainConfig) (vm.Tracer, error) {
    info, exists := Registry[name]
    if exists {
		return info.factory(ctx, config, chainConfig)
    }
	if jsEvalFactory != nil {
        return jsEvalFactory(name, ctx, config, chainConfig)
    }
	return nil, fmt.Errorf("unknown tracer: %s", name)
}

func IsJS(name string) bool {
    if info, exists := Registry[name]; exists {
        return info.isJS
    }
    return false
}

var jsEvalFactory func(string, *Context, json.RawMessage, *params.ChainConfig) (vm.Tracer, error)

func RegisterJSEval(factory func(string, *Context, json.RawMessage, *params.ChainConfig) (vm.Tracer, error)) {
    jsEvalFactory = factory
}