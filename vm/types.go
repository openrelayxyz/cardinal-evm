package vm

import (
	"encoding/json"
	"fmt"

	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-types"
)

type BlockNumberOrHash struct {
	BlockNumber      *rpc.BlockNumber `json:"blockNumber,omitempty"`
	BlockHash        *types.Hash  `json:"blockHash,omitempty"`
	RequireCanonical bool         `json:"requireCanonical,omitempty"`
}

func (bnh *BlockNumberOrHash) UnmarshalJSON(data []byte) error {
	type erased BlockNumberOrHash
	e := erased{}
	err := json.Unmarshal(data, &e)
	if err == nil {
		if e.BlockNumber != nil && e.BlockHash != nil {
			return fmt.Errorf("cannot specify both BlockHash and BlockNumber, choose one or the other")
		}
		bnh.BlockNumber = e.BlockNumber
		bnh.BlockHash = e.BlockHash
		bnh.RequireCanonical = e.RequireCanonical
		return nil
	}
	h := &types.Hash{}
	if err := json.Unmarshal(data, h); err == nil {
		bnh.BlockHash = h
		return nil
	}
	bn := new(rpc.BlockNumber)
	if err := json.Unmarshal(data, bn); err == nil {
		bnh.BlockNumber = bn
	}
	return err
}

func (bnh *BlockNumberOrHash) Number() (rpc.BlockNumber, bool) {
	if bnh.BlockNumber != nil {
		return *bnh.BlockNumber, true
	}
	return rpc.BlockNumber(0), false
}

func (bnh *BlockNumberOrHash) Hash() (types.Hash, bool) {
	if bnh.BlockHash != nil {
		return *bnh.BlockHash, true
	}
	return types.Hash{}, false
}

func BlockNumberOrHashWithNumber(blockNr rpc.BlockNumber) BlockNumberOrHash {
	return BlockNumberOrHash{
		BlockNumber:      &blockNr,
		BlockHash:        nil,
		RequireCanonical: false,
	}
}

func BlockNumberOrHashWithHash(hash types.Hash, canonical bool) BlockNumberOrHash {
	return BlockNumberOrHash{
		BlockNumber:      nil,
		BlockHash:        &hash,
		RequireCanonical: canonical,
	}
}
