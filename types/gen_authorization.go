// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package types

import (
	"encoding/json"
	"errors"

	"github.com/holiman/uint256"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

var _ = (*authorizationMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (a Authorization) MarshalJSON() ([]byte, error) {
	type Authorization struct {
		ChainID hexutil.Uint64 `json:"chainId" gencodec:"required"`
		Address common.Address `json:"address" gencodec:"required"`
		Nonce   hexutil.Uint64 `json:"nonce" gencodec:"required"`
		V       hexutil.Uint64 `json:"v" gencodec:"required"`
		R       uint256.Int    `json:"r" gencodec:"required"`
		S       uint256.Int    `json:"s" gencodec:"required"`
	}
	var enc Authorization
	enc.ChainID = hexutil.Uint64(a.ChainID)
	enc.Address = a.Address
	enc.Nonce = hexutil.Uint64(a.Nonce)
	enc.V = hexutil.Uint64(a.V)
	enc.R = a.R
	enc.S = a.S
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (a *Authorization) UnmarshalJSON(input []byte) error {
	type Authorization struct {
		ChainID *hexutil.Uint64 `json:"chainId" gencodec:"required"`
		Address *common.Address `json:"address" gencodec:"required"`
		Nonce   *hexutil.Uint64 `json:"nonce" gencodec:"required"`
		V       *hexutil.Uint64 `json:"v" gencodec:"required"`
		R       *uint256.Int    `json:"r" gencodec:"required"`
		S       *uint256.Int    `json:"s" gencodec:"required"`
	}
	var dec Authorization
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ChainID == nil {
		return errors.New("missing required field 'chainId' for Authorization")
	}
	a.ChainID = uint64(*dec.ChainID)
	if dec.Address == nil {
		return errors.New("missing required field 'address' for Authorization")
	}
	a.Address = *dec.Address
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' for Authorization")
	}
	a.Nonce = uint64(*dec.Nonce)
	if dec.V == nil {
		return errors.New("missing required field 'v' for Authorization")
	}
	a.V = uint8(*dec.V)
	if dec.R == nil {
		return errors.New("missing required field 'r' for Authorization")
	}
	a.R = *dec.R
	if dec.S == nil {
		return errors.New("missing required field 's' for Authorization")
	}
	a.S = *dec.S
	return nil
}
