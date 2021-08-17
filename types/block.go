package types

import (
  "math/big"
  "github.com/openrelayxyz/cardinal-types"
  "github.com/openrelayxyz/cardinal-evm/common"
)

type Header struct {
	ParentHash  types.Hash
	UncleHash   types.Hash
	Coinbase    common.Address
	Root        types.Hash
	TxHash      types.Hash
	ReceiptHash types.Hash
	Bloom       [2048]byte
	Difficulty  *big.Int
	Number      *big.Int
	GasLimit    uint64
	GasUsed     uint64
	Time        uint64
	Extra       []byte
	MixDigest   types.Hash
	Nonce       [8]byte
	BaseFee *big.Int
}
