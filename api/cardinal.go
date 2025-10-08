package api

import (
	"github.com/openrelayxyz/cardinal-evm/params"
)

type CardinalAPI struct {
	config *params.ChainConfig
}

func NewCardinalAPI(chaincfg *params.ChainConfig) *CardinalAPI {
	return &CardinalAPI{config: chaincfg}
}

func (s *CardinalAPI) ForkReady(forkName string) int {
	result := -1
	switch {
	case "osaka":
		
	}
	return result
}
