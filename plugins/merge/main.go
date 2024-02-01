package main

import (
	"fmt"
	"math/big"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted"
	"github.com/openrelayxyz/plugeth-utils/restricted/hexutil"
	"github.com/openrelayxyz/cardinal-types/metrics"
)

var (
	log core.Logger
	postMerge bool
	backend restricted.Backend
	gethWeightGauge = metrics.NewMajorGauge("/geth/weight")
	stack core.Node
	chainid int64
)

type numLookup struct{
	Number hexutil.Big `json:"number"`
}

func getSafeFinalized() (*big.Int, *big.Int) {
	client, err := stack.Attach()
	if err != nil {
		log.Warn("Could not stack.Attach()", "err", err)
		return nil, nil
	}
	var snl, fnl numLookup
	if err := client.Call(&snl, "eth_getBlockByNumber", "safe", false); err != nil {
		log.Warn("Could not get safe block", "err", err)
	}
	if err := client.Call(&fnl, "eth_getBlockByNumber", "finalized", false); err != nil {
		log.Warn("Could not get finalized block", "err", err)
	}
	return snl.Number.ToInt(), fnl.Number.ToInt()
}

func Initialize(ctx core.Context, loader core.PluginLoader, logger core.Logger) {
	log = logger
	log.Info("Cardinal EVM Merge plugin initializing")
}

func InitializeNode(s core.Node, b restricted.Backend) {
	backend = b
	stack = s
	chainid = b.ChainConfig().ChainID.Int64()
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
	snum, fnum := getSafeFinalized()
	if snum != nil {
		updates[fmt.Sprintf("c/%x/n/safe", chainid)] = snum.Bytes()
	}
	if fnum != nil {
		updates[fmt.Sprintf("c/%x/n/finalized", chainid)] = fnum.Bytes()
	}
	// After the merge, the td of a block stops increasing, but certain elements
	// of  Cardinal still needs a weight for evaluating block ordering. The
	// convention for this is to add the block number to the final total
	// difficulty to choose a weight.
	weight.Add(weight, big.NewInt(number))
	gethWeightGauge.Update(new(big.Int).Div(weight, big.NewInt(10000000000000000)).Int64())
}
