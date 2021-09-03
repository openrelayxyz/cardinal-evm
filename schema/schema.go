package schema

import (
	"fmt"
	"github.com/openrelayxyz/cardinal-evm/crypto"
)

// AccountData returns the Cardinal Storage key containing the account data for
// the specified address
func AccountData(chainid int64, address []byte) []byte {
	return []byte(fmt.Sprintf("c/%x/a/%x/d", chainid, crypto.Keccak256Hash(address)))
}

// AccountCode returns the Cardinal Storage key containing the account code for
// the specified address.
func AccountCode(chainid int64, codeHash []byte) []byte {
	// TODO: Evaluate code storage path.
	// We can look up by address even if we've never had to load the account
	// information to get the hash, which could lead to faster code lookups.
	// That said, I'm not sure how often such an occurrence would arise. Within
	// the EVM I'm not sure it would ever attempt to load code it hasn't already
	// loaded the account for. The downside of this storage approach is that we
	// duplicate code storage when multiple accounts have the same code. Storing
	// by hash would deduplicate the stored code, saving space at the possible
	// cost of extra lookups.
	return []byte(fmt.Sprintf("c/%x/c/%x", chainid, codeHash))
}

// AccountStorage returns the Cardinal Storage key containing the data stored
// by the specified address at the specified storage entry.
func AccountStorage(chainid int64, address []byte, storage []byte) []byte {
	return []byte(fmt.Sprintf("c/%x/a/%x/s/%x", chainid, crypto.Keccak256Hash(address), storage))
}

// BlockHeader returns the Cardinal Storage key containing the RLP encoded
// header for the block at the indicated hash.
func BlockHeader(chainid int64, hash []byte) []byte {
	return []byte(fmt.Sprintf("c/%x/b/%x/h", chainid, hash))
}

// BlockTotalDifficulty returns the Cardinal Storage key containing the total
// difficulty for the block at the indicated hash.
func BlockTotalDifficulty(chainid int64, hash []byte) []byte {
	return []byte(fmt.Sprintf("c/%x/b/%x/d", chainid, hash))
}

// BlockNumberToHash returns the Cardinal Storage key containing the hash of
// the block at the specified number
func BlockNumberToHash(chainid int64, number int64) []byte {
	return []byte(fmt.Sprintf("c/%x/n/%x", chainid, number))
}

// Transaction returns the Cardinal Storage key containing the RLP encoded
// transaction in the specified block at the specified index.
func Transaction(chainid int64, blockHash []byte, txIndex int64) []byte {
	return []byte(fmt.Sprintf("c/%x/b/%x/t/%x", chainid, blockHash, txIndex))
}

// Receipt returns the Cardinal Storage key containing the encoded
// transaction receipt in the specified block at the specified index. This
// receipt does not include logs.
func Receipt(chainid int64, blockHash []byte, txIndex int64) []byte {
	return []byte(fmt.Sprintf("c/%x/b/%x/r/%x", chainid, blockHash, txIndex))
}

// Log returns the Cardinal Storage key containing the encoded log in the
// specified transaction at the specified index of the specified block.
func Log(chainid int64, blockHash []byte, txIndex, logIndex int64) []byte {
	return []byte(fmt.Sprintf("c/%x/b/%x/l/%x/%x", chainid, blockHash, txIndex, logIndex))
}

// TransactionHash returns the Cardinal Storage key containing the block number
// and index of the transaction specified by hash.
func TransactionHash(chainid, txHash []byte) []byte {
	return []byte(fmt.Sprintf("c/%x/t/%x", chainid, txHash))
}
