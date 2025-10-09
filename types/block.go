package types

import (
	"math/big"
	"sync/atomic"
	"time"
	"slices"
	"encoding/binary"

	"github.com/ethereum/go-verkle"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-types"
)

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

// ExecutionWitness represents the witness + proof used in a verkle context,
// to provide the ability to execute a block statelessly.
type ExecutionWitness struct {
	StateDiff   verkle.StateDiff    `json:"stateDiff"`
	VerkleProof *verkle.VerkleProof `json:"verkleProof"`
}

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	ParentHash  types.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   types.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"`
	// Root        types.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      types.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash types.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
	Time        uint64         `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   types.Hash    `json:"mixHash"`
	Nonce       BlockNonce     `json:"nonce"`

	// BaseFee was added by EIP-1559 and is ignored in legacy headers.
	BaseFee *big.Int `json:"baseFeePerGas" rlp:"optional"`

	// WithdrawalsHash was added by EIP-4895 and is ignored in legacy headers.
	WithdrawalsHash *types.Hash `json:"withdrawalsRoot" rlp:"optional"`

	// BlobGasUsed was added by EIP-4844 and is ignored in legacy headers.
	BlobGasUsed *uint64 `json:"blobGasUsed" rlp:"optional"`

	// ExcessBlobGas was added by EIP-4844 and is ignored in legacy headers.
	ExcessBlobGas *uint64 `json:"excessBlobGas" rlp:"optional"`

	// ParentBeaconRoot was added by EIP-4788 and is ignored in legacy headers.
	ParentBeaconRoot *types.Hash `json:"parentBeaconBlockRoot" rlp:"optional"`

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

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions []*Transaction
	Uncles       []*Header
	Withdrawals  []*Withdrawal `rlp:"optional"`
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// NewBlock creates a new block. The input data is copied, changes to header and to the
// field values will not affect the block.
//
// The body elements and the receipts are used to recompute and overwrite the
// relevant portions of the header.
//
// The receipt's bloom must already calculated for the block's bloom to be
// correctly calculated.
func NewBlock(header *Header, body *Body, receipts []*Receipt, hasher TrieHasher) *Block {
	if body == nil {
		body = &Body{}
	}
	var (
		b           = NewBlockWithHeader(header)
		txs         = body.Transactions
		uncles      = body.Uncles
		withdrawals = body.Withdrawals
	)

	if len(txs) == 0 {
		b.header.TxHash = EmptyTxsHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs), hasher)
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyReceiptsHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts), hasher)
		// Receipts must go through MakeReceipt to calculate the receipt's bloom
		// already. Merge the receipt's bloom together instead of recalculating
		// everything.
		b.header.Bloom = MergeBloom(receipts)
	}

	if len(uncles) == 0 {
		b.header.UncleHash = EmptyUncleHash
	} else {
		b.header.UncleHash = CalcUncleHash(uncles)
		b.uncles = make([]*Header, len(uncles))
		for i := range uncles {
			b.uncles[i] = CopyHeader(uncles[i])
		}
	}

	if withdrawals == nil {
		b.header.WithdrawalsHash = nil
	} else if len(withdrawals) == 0 {
		b.header.WithdrawalsHash = &EmptyWithdrawalsHash
		b.withdrawals = Withdrawals{}
	} else {
		hash := DeriveSha(Withdrawals(withdrawals), hasher)
		b.header.WithdrawalsHash = &hash
		b.withdrawals = slices.Clone(withdrawals)
	}

	return b
}

func CalcUncleHash(uncles []*Header) types.Hash {
	if len(uncles) == 0 {
		return EmptyUncleHash
	}
	return rlpHash(uncles)
}