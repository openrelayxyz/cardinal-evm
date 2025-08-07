package types

import (
	"math/big"
	"sync/atomic"
	"time"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-types"
)

var (
	// EmptyUncleHash is the known hash of the empty uncle set.
	EmptyUncleHash = rlpHash([]*Header(nil)) // 1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347

)

type Header struct {
	ParentHash      types.Hash
	UncleHash       types.Hash
	Coinbase        common.Address
	Root            types.Hash
	TxHash          types.Hash
	ReceiptHash     types.Hash
	Bloom           [256]byte
	Difficulty      *big.Int
	Number          *big.Int
	GasLimit        uint64
	GasUsed         uint64
	Time            uint64
	Extra           []byte
	MixDigest       types.Hash
	Nonce           [8]byte
	BaseFee         *big.Int    `rlp:"optional"`
	WithdrawalsHash *types.Hash `rlp:"optional"`
	// BlobGasUsed was added by EIP-4844 and is ignored in legacy headers.
	BlobGasUsed *uint64 `rlp:"optional"`

	// ExcessBlobGas was added by EIP-4844 and is ignored in legacy headers.
	ExcessBlobGas *uint64 `rlp:"optional"`

	// BeaconRoot was added by EIP-4788 and is ignored in legacy headers.
	BeaconRoot *types.Hash `rlp:"optional"`

	// RequestsHash was added by EIP-7685 and is ignored in legacy headers.
	RequestsHash *types.Hash `json:"requestsHash" rlp:"optional"`
}

type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions
	withdrawals  Withdrawals

	// caches
	hash atomic.Pointer[types.Hash]
	size atomic.Uint64

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// Header returns the block header (as a copy).
func (b *Block) Header() *Header {
	return CopyHeader(b.header)
}

// CopyHeader creates a deep copy of a block header.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if h.BaseFee != nil {
		cpy.BaseFee = new(big.Int).Set(h.BaseFee)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	if h.WithdrawalsHash != nil {
		cpy.WithdrawalsHash = new(types.Hash)
		*cpy.WithdrawalsHash = *h.WithdrawalsHash
	}
	if h.ExcessBlobGas != nil {
		cpy.ExcessBlobGas = new(uint64)
		*cpy.ExcessBlobGas = *h.ExcessBlobGas
	}
	if h.BlobGasUsed != nil {
		cpy.BlobGasUsed = new(uint64)
		*cpy.BlobGasUsed = *h.BlobGasUsed
	}
	if h.RequestsHash != nil {
		cpy.RequestsHash = new(types.Hash)
		*cpy.RequestsHash = *h.RequestsHash
	}
	return &cpy
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() types.Hash {
	return rlpHash(h)
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}