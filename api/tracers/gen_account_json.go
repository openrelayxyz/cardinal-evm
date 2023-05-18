// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package tracers

import (
	"encoding/json"
	"math/big"

	"github.com/openrelayxyz/cardinal-types/hexutil"
	ctypes "github.com/openrelayxyz/cardinal-types"
)

var _ = (*accountMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (a account) MarshalJSON() ([]byte, error) {
	type account struct {
		Balance *hexutil.Big                `json:"balance,omitempty"`
		Code    hexutil.Bytes               `json:"code,omitempty"`
		Nonce   uint64                      `json:"nonce,omitempty"`
		Storage map[ctypes.Hash]ctypes.Hash `json:"storage,omitempty"`
	}
	var enc account
	enc.Balance = (*hexutil.Big)(a.Balance)
	enc.Code = a.Code
	enc.Nonce = a.Nonce
	enc.Storage = a.Storage
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (a *account) UnmarshalJSON(input []byte) error {
	type account struct {
		Balance *hexutil.Big                `json:"balance,omitempty"`
		Code    *hexutil.Bytes              `json:"code,omitempty"`
		Nonce   *uint64                     `json:"nonce,omitempty"`
		Storage map[ctypes.Hash]ctypes.Hash `json:"storage,omitempty"`
	}
	var dec account
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Balance != nil {
		a.Balance = (*big.Int)(dec.Balance)
	}
	if dec.Code != nil {
		a.Code = *dec.Code
	}
	if dec.Nonce != nil {
		a.Nonce = *dec.Nonce
	}
	if dec.Storage != nil {
		a.Storage = dec.Storage
	}
	return nil
}
