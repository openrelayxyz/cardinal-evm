// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-types"
	"math/big"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = types.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     []byte
	CodeHash []byte
}

func (ac *Account) Copy() *Account {
	if ac == nil {
		return &Account{
			Nonce:    0,
			Balance:  new(big.Int),
			Root:     emptyRoot.Bytes(),
			CodeHash: emptyCode.Bytes(),
		}
	}
	result := &Account{
		Nonce:    ac.Nonce,
		Balance:  new(big.Int),
		Root:     make([]byte, len(ac.Root)),
		CodeHash: make([]byte, len(ac.CodeHash)),
	}
	if ac.Balance != nil {
		result.Balance.Set(ac.Balance)
	}
	if ac.Root != nil {
		copy(result.Root[:], ac.Root)
	}
	if ac.CodeHash != nil {
		copy(result.CodeHash[:], ac.CodeHash)
	}
	return result
}

// FullAccount decodes the data on the 'slim RLP' format and return
// the consensus format account.
func FullAccount(data []byte) (Account, error) {
	var account Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return Account{}, err
	}
	if len(account.Root) == 0 {
		account.Root = emptyRoot[:]
	}
	if len(account.CodeHash) == 0 {
		account.CodeHash = emptyCode[:]
	}
	return account, nil
}
