// The calls in this package don't rely on any kind of state, and are provided
// solely because some applications will expect them to be implemented.

package api

import (
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-evm/common"
)


// ProtocolVersion indicates the Ethereum protocol version (used in p2p
// connections)
func (api *PublicBlockChainAPI) ProtocolVersion() (hexutil.Uint64) {
	return 66
}

// Syncing indicates whether a node is syncing
func (api *PublicBlockChainAPI) Syncing() (bool) {
	return false
}
// Mining indicates whether a node is mining
func (api *PublicBlockChainAPI) Mining() (bool) {
	return false
}

// Hashrate indicates the hashing power of the node
func (api *PublicBlockChainAPI) Hashrate() (hexutil.Uint64) {
	return 0
}

// Coinbase indicates the address the address the node is mining ETH into
func (api *PublicBlockChainAPI) Coinbase() (common.Address) {
	return common.Address{}
}

// Accounts indicates the addresses this node can sign for
func (api *PublicBlockChainAPI) Accounts() ([]common.Address) {
	return []common.Address{}
}
