package main

import (
	"math/big"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted"
	"github.com/openrelayxyz/cardinal-types/metrics"
)

var (
	log core.Logger
	postMerge bool
	backend restricted.Backend
	gethWeightGauge = metrics.NewMajorGauge("/geth/weight")
)

func Initialize(ctx core.Context, loader core.PluginLoader, logger core.Logger) {
	log = logger
	log.Info("Cardinal EVM Merge plugin initializing")
}

func InitializeNode(stack core.Node, b restricted.Backend) {
	backend = b
}

func CardinalAddBlockHook(number int64, hash, parent ctypes.Hash, weight *big.Int, updates map[string][]byte, deletes map[string]struct{}) {
	if !postMerge {
		v, _ := backend.ChainDb().Get([]byte("eth2-transition"))
		if len(v) > 0 {
			postMerge = true
		} else {
			// Not yet post merge, we don't want to make any modifications
			gethWeightGauge.Update(new(big.Int).Div(weight, big.NewInt(10000000000000000)).Int64())
			return
		}
	}
	// After the merge, the td of a block stops increasing, but certain elements
	// of  Cardinal still needs a weight for evaluating block ordering. The
	// convention for this is to add the block number to the final total
	// difficulty to choose a weight.
	weight.Add(weight, big.NewInt(number))
	gethWeightGauge.Update(new(big.Int).Div(weight, big.NewInt(10000000000000000)).Int64())
}
