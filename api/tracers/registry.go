package tracers

import (
	"encoding/json"
	"fmt"
	"math/big"

	log "github.com/inconshreveable/log15"
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

type TracerFactory func(config json.RawMessage, chainConfig *params.ChainConfig) (vm.Tracer, error)

var Registry = make(map[string]TracerFactory)

func Register(name string, factory TracerFactory) {
    Registry[name] = factory
}

func New(name string, config json.RawMessage, chainConfig *params.ChainConfig) (vm.Tracer, error) {
    factory, exists := Registry[name]
    if !exists {
		log.Error("tracer %s not found in registry", name)
        return nil, fmt.Errorf("unknown tracer: %s", name)
    }
    return factory(config, chainConfig)
}