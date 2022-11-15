package engine

import (
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/params"
	log "github.com/inconshreveable/log15"
)

var (
	extraSeal   = crypto.SignatureLength
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")
)

type Engine interface{
	Author(*types.Header) common.Address
}

type ETHashEngine struct{}

func (*ETHash) Author(h *types.Header) common.Address {
	return h.Coinbase
}

type BeaconEngine struct{}

func (*Beacon) Author(h *types.Header) common.Address {
	return h.Coinbase
}

type CliqueEngine struct{}

func (*Clique) Author(h *types.Header) common.Address {
	author, err := ecrecoverClique(h)
	if err != nil {
		log.Warn("Error recovering author", "num", h.Number, "hash", h.Hash(), err", err)
	}
	return author
}


func ecrecoverClique(header *types.Header) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(sealHashClique(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	return signer, nil
}

func sealHashClique(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.(crypto.KeccakState).Read(hash[:])
	return hash
}

type BorEngine struct{}

func (*Bor) Author(h *types.Header) common.Address {
	author, err := ecrecoverBor(h)
	if err != nil {
		log.Warn("Error recovering author", "num", h.Number, "hash", h.Hash(), err", err)
	}
	return author
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecoverBor(header *types.Header) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(sealHashBor(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	return signer, nil
}

// SealHash returns the hash of a block prior to it being sealed.
func sealHashBor(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header, c)
	hasher.Sum(hash[:0])
	return hash
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}

func Author(h *types.Header, e params.Engine) common.Address {
	switch e {
	case params.ETHashEngine:
		return (&ETHashEngine{}).Author(h)
	case params.BeaconEngine:
		return (&BeaconEngine{}).Author(h)
	case params.CliqueEngine:
		return (&CliqueEngine{}).Author(h)
	case params.BorEngine:
		return (&BorEngine{}).Author(h)
	default:
		log.Warn("Unknown engine", "engine", e)
		return (&ETHashEngine{}).Author(h)
	}
}
