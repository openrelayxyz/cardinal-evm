package api

import (
	log "github.com/inconshreveable/log15"

	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-evm/params"
)

type CardinalAPI struct {
	config *params.ChainConfig
	evmmgr  *vm.EVMManager
}

func NewCardinalAPI(evmmgr *vm.EVMManager, chaincfg *params.ChainConfig) *CardinalAPI {
	return &CardinalAPI{
		config: chaincfg,
		evmmgr: chaincfg,
	}
}

func (api *CardinalAPI) ForkReady(forkName string) int {
	nullCase := -1
	switch {
	case "osaka":
		var latestTime uint64
		if err := api.evmmgr.View(ctx, func(header *types.Header) {
			latestTime := header.Time
		}); err != nil {
			log.Error("error encountered calling for header, forkready", "err", err)
			return nullCase
		}
		api.config.OsakaTime 
	}
	return result
}
