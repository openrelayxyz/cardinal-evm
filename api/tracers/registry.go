package tracers

import (
	"encoding/json"
	"math/big"
	"github.com/openrelayxyz/cardinal-types"
)

type Context struct {
	BlockHash   types.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int    // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int         // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      types.Hash // Hash of the transaction being traced (zero if dangling call)
}

type registry map[string]func(*Context, json.RawMessage) (TracerResult, error)

var Registry registry

func init() {
	if Registry == nil {
		Registry = make(map[string]func(*Context, json.RawMessage) (TracerResult, error))
	}
}