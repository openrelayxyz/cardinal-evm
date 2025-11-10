package api

import (
	"context"
	"reflect"

	log "github.com/inconshreveable/log15"

	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
)

type CardinalAPI struct {
	mgr  *vm.EVMManager
	config *params.ChainConfig
}

func NewCardinalAPI(evmmgr *vm.EVMManager, chaincfg *params.ChainConfig) *CardinalAPI {
	return &CardinalAPI{
		mgr: evmmgr,
		config: chaincfg,
	}
}

func (api *CardinalAPI) ForkReady(ctx context.Context, forkName string) int {
	result := -1
	switch forkName {
	case "osaka":
		var latestTime uint64
		if err := api.mgr.View(ctx, func(header *types.Header) {
			latestTime = header.Time
		}); err != nil {
			log.Error("error encountered calling for header, forkready", "err", err)
			return result
		}
		cfg := reflect.ValueOf(api.config).Type()
		if _, ok := cfg.FieldByName("OsakaTime"); ok {
			if oTime := api.config.OsakaTime; oTime != nil {
				osaka := *oTime
				if latestTime >= osaka.Uint64() {
					result = 2
				} else {
					result = 1
				}
			} else {
				result = 0
			}
		} 
	}
	return result
}
