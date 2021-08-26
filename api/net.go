package api


import (
	"math/big"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

type NetAPI struct{
	NetVersion *big.Int
}

// Listening returns an indication if the node is listening for network connections.
func (s *NetAPI) Listening() bool {
	return true // always listening
}

// PeerCount returns the number of connected peers
func (s *NetAPI) PeerCount() hexutil.Uint {
	return 0
}

// Version returns the current ethereum protocol version.
func (s *NetAPI) Version() string {
	return s.NetVersion.String()
}
