package api

import (
	"runtime"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/build"
)

type Web3API struct {}

func (s *Web3API) ClientVersion() string {
	version := build.Version
	if version == "" { version = "unset-use-make-to-build"}
	name := "CardinalEVM"
	name += "/" + version
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

// Sha3 applies the ethereum sha3 implementation on the input.
// It assumes the input is hex encoded.
func (s *Web3API) Sha3(input hexutil.Bytes) hexutil.Bytes {
	return crypto.Keccak256(input)
}
