// Copyright 2016 The go-ethereum Authors
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

package vm

import (
	"math/big"
	"testing"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-types"
	// "github.com/ethereum/go-ethereum/core/state"
	// "github.com/holiman/uint256"
	"github.com/openrelayxyz/cardinal-evm/params"
)

type dummyContractRef struct {
	calledForEach bool
}

func (dummyContractRef) Address() common.Address    { return common.Address{} }
func (dummyContractRef) Value() *big.Int            { return new(big.Int) }
func (dummyContractRef) SetCode(types.Hash, []byte) {}

// func (d *dummyContractRef) ForEachStorage(callback func(key, value types.Hash) bool) {
// 	d.calledForEach = true
// }
func (d *dummyContractRef) SubBalance(amount *big.Int) {}
func (d *dummyContractRef) AddBalance(amount *big.Int) {}
func (d *dummyContractRef) SetBalance(*big.Int)        {}
func (d *dummyContractRef) SetNonce(uint64)            {}
func (d *dummyContractRef) Balance() *big.Int          { return new(big.Int) }

type dummyStatedb struct {
	state.StateDB
}

func (*dummyStatedb) GetRefund() uint64 { return 1337 }

func TestStoreCapture(t *testing.T) {
	var (
		env      = NewEVM(BlockContext{}, TxContext{}, &dummyStatedb{}, params.TestChainConfig, Config{})
		logger   = NewStructLogger(nil)
		contract = NewContract(&dummyContractRef{}, &dummyContractRef{}, new(big.Int), 0)
		scope    = &ScopeContext{
			Memory:   NewMemory(),
			Stack:    newstack(),
			Contract: contract,
		}
	)
	scope.Stack.push(NewInt(1))
	scope.Stack.push(new(Int))
	var index types.Hash
	logger.CaptureState(env, 0, SSTORE, 0, 0, scope, nil, 0, nil)
	if len(logger.storage[contract.Address()]) == 0 {
		t.Fatalf("expected exactly 1 changed value on address %x, got %d", contract.Address(),
			len(logger.storage[contract.Address()]))
	}
	exp := types.BigToHash(big.NewInt(1))
	if logger.storage[contract.Address()][index] != exp {
		t.Errorf("expected %x, got %x", exp, logger.storage[contract.Address()][index])
	}
}
