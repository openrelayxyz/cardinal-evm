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
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-storage/current"
	"github.com/openrelayxyz/cardinal-storage/db/mem"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"math/big"
)

type journalEntry struct {
	addr   *common.Address
	revert func(*stateDB)
}

type stateDB struct {
	tx         storage.Transaction
	journal    []journalEntry
	state      map[common.Address]*stateObject
	transient map[common.Address]Storage
	chainid    int64
	refund     uint64
	accessList *accessList
	alcalc     bool
	logs    map[ctypes.Hash][]*types.Log
	thash   ctypes.Hash
	txIndex int
	logSize uint
}

func NewStateDB(tx storage.Transaction, chainid int64) StateDB {
	return &stateDB{
		tx:         tx,
		state:      make(map[common.Address]*stateObject),
		journal:    []journalEntry{},
		chainid:    chainid,
		accessList: newAccessList(),
		transient:  make(map[common.Address]Storage),
	}
}

type StatedbManager struct {
	Storage storage.Storage
	Chainid int64
}

func (sdbm *StatedbManager) View(h ctypes.Hash, fn func(storage.Transaction, StateDB) error) error {
	return sdbm.Storage.View(h, func(tx storage.Transaction) error {
		return fn(tx, NewStateDB(tx, sdbm.Chainid))
	})
}

func NewMemStateDB(chainid, reorgDepth int64) *StatedbManager {
	mdb := mem.NewMemoryDatabase(4)
	// TODO: Support whitelist functionality
	return &StatedbManager{Storage: current.New(mdb, reorgDepth, nil), Chainid: chainid}
}

func (sdb *stateDB) kv() []storage.KeyValue {
	result := []storage.KeyValue{}
	for _, sobj := range sdb.state {
		result = append(result, sobj.kv(sdb.chainid)...)
	}
	return result
}

func (sdb *stateDB) Copy() StateDB {
	state := make(map[common.Address]*stateObject)
	for addr, sobj := range sdb.state {
		state[addr] = sobj.copy()
	}
	transient := make(map[common.Address]Storage)
	for addr, sobj := range sdb.transient {
		transient[addr] = sobj
	}
	journal := make([]journalEntry, len(sdb.journal))
	copy(journal[:], sdb.journal[:])
	return &stateDB{
		tx:         sdb.tx,
		journal:    journal,
		state:      state,
		transient:  transient,
		chainid:    sdb.chainid,
		refund:     sdb.refund,
		accessList: sdb.accessList.Copy(),
	}
}

func (sdb *stateDB) ALCalcCopy() StateDB {
	copy := sdb.Copy().(*stateDB)
	copy.alcalc = true
	return copy
}

func (sdb *stateDB) Finalise() {
	for _, sobj := range sdb.state {
		sobj.finalise()
	}
	sdb.accessList = newAccessList()
	sdb.transient = make(map[common.Address]Storage)
	sdb.refund = 0
}

func (sdb *stateDB) getAccount(addr common.Address) *stateObject {
	if sobj, ok := sdb.state[addr]; ok {
		return sobj
	}
	sdb.state[addr] = &stateObject{
		address: addr,
		dirty:   make(Storage),
		clean:   make(Storage),
	}
	return sdb.state[addr]
}

func (sdb *stateDB) CreateAccount(addr common.Address) {
	prev := sdb.getAccount(addr)

	sdb.state[addr] = &stateObject{
		address: addr,
		account: &Account{},
		dirty:   make(Storage),
		clean:   make(Storage),
		created: true,
	}
	sdb.journal = append(sdb.journal, journalEntry{&addr, func(sdb *stateDB) { sdb.state[addr] = prev }})
	if !prev.deleted && !prev.suicided {
		if prev.loadAccount(sdb.tx, sdb.chainid) {
			sdb.state[addr].addBalance(prev.getBalance())
		}

	}
}

func (sdb *stateDB) SubBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.subBalance(amount))
}
func (sdb *stateDB) AddBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.addBalance(amount))
}
func (sdb *stateDB) GetBalance(addr common.Address) *big.Int {
	sobj := sdb.getAccount(addr)
	if !sobj.loadAccount(sdb.tx, sdb.chainid) {
		return common.Big0
	}
	return sobj.getBalance()
}
func (sdb *stateDB) GetNonce(addr common.Address) uint64 {
	sobj := sdb.getAccount(addr)
	return sobj.getNonce(sdb.tx, sdb.chainid)
}
func (sdb *stateDB) SetNonce(addr common.Address, nonce uint64) {
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.setNonce(nonce))
}
func (sdb *stateDB) GetCodeHash(addr common.Address) ctypes.Hash {
	sobj := sdb.getAccount(addr)
	return sobj.getCodeHash(sdb.tx, sdb.chainid)
}

func (sdb *stateDB) GetCode(addr common.Address) []byte {
	sobj := sdb.getAccount(addr)
	return sobj.getCode(sdb.tx, sdb.chainid)
}
func (sdb *stateDB) SetCode(addr common.Address, code []byte) {
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.setCode(code))
}
func (sdb *stateDB) GetCodeSize(addr common.Address) int {
	sobj := sdb.getAccount(addr)
	return len(sobj.getCode(sdb.tx, sdb.chainid))
}
func (sdb *stateDB) AddRefund(amount uint64) {
	old := sdb.refund
	sdb.refund += amount
	sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.refund = old }})
}
func (sdb *stateDB) SubRefund(amount uint64) {
	old := sdb.refund
	sdb.refund -= amount
	sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.refund = old }})
}
func (sdb *stateDB) GetRefund() uint64 { return sdb.refund }
func (sdb *stateDB) GetCommittedState(addr common.Address, storage ctypes.Hash) ctypes.Hash {
	sobj := sdb.getAccount(addr)
	return sobj.getCommittedState(sdb.tx, sdb.chainid, crypto.Keccak256Hash(storage.Bytes()))
}
func (sdb *stateDB) GetState(addr common.Address, storage ctypes.Hash) ctypes.Hash {
	sobj := sdb.getAccount(addr)
	data := sobj.getState(sdb.tx, sdb.chainid, crypto.Keccak256Hash(storage.Bytes()))
	// log.Debug("Got state", "addr", addr, "hashaddr", crypto.Keccak256Hash(addr.Bytes()), "storage", storage, "data", data)
	return data
}
func (sdb *stateDB) SetState(addr common.Address, storage, data ctypes.Hash) {
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.setState(crypto.Keccak256Hash(storage.Bytes()), data))
}
func (sdb *stateDB) SetTransientState(addr common.Address, storage, data ctypes.Hash) {
	s, ok := sdb.transient[addr]
	if !ok {
		s = make(Storage)
		sdb.transient[addr] = s
	}
	old := s[storage]
	sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.transient[addr][storage] = old }})
	s[storage] = data
}
func (sdb *stateDB) GetTransientState(addr common.Address, storage ctypes.Hash) ctypes.Hash {
	s, ok := sdb.transient[addr]
	if !ok {
		return ctypes.Hash{}
	}
	return s[storage]
}
func (sdb *stateDB) SetStorage(addr common.Address, storage map[ctypes.Hash]ctypes.Hash) {
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.setStorage(storage))
}
func (sdb *stateDB) SetBalance(addr common.Address, balance *big.Int) {
	sobj := sdb.getAccount(addr)
	sdb.journal = append(sdb.journal, sobj.setBalance(balance))
	log.Error("balance inside SetBalance", "b", balance, "address", addr.Hex())
}

func (sdb *stateDB) Suicide(addr common.Address) bool {
	sobj := sdb.getAccount(addr)
	ok, je := sobj.suicide()
	if ok {
		sdb.journal = append(sdb.journal, *je)
	}
	return ok
}
func (sdb *stateDB) SelfDestruct6780(addr common.Address) {
	sobj := sdb.getAccount(addr)
	if sobj.created {
		sdb.Suicide(addr)
		return
	}
}
func (sdb *stateDB) HasSuicided(addr common.Address) bool {
	sobj := sdb.getAccount(addr)
	return sobj.suicided
}

// Exist reports whether the given account exists in state.
// Notably this should also return true for suicided accounts.
// Cardinal note: I believe "suicided accounts" refers to accounts that
// suicided within the curent transaction, not suicided ever

func (sdb *stateDB) Exist(addr common.Address) bool {
	sobj := sdb.getAccount(addr)
	// I'm not 100% certain this is the correct interpretation of "exists"
	return !sobj.deleted && !sobj.empty(sdb.tx, sdb.chainid)
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (sdb *stateDB) Empty(addr common.Address) bool {
	sobj := sdb.getAccount(addr)
	return sobj.deleted || sobj.suicided || sobj.empty(sdb.tx, sdb.chainid)
}
func (sdb *stateDB) PrepareAccessList(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	if rules.IsBerlin {
		sdb.AddAddressToAccessList(sender)
		if dst != nil {
			sdb.AddAddressToAccessList(*dst)
			// If it's a create-tx, the destination will be added inside evm.create
		}
		for _, addr := range precompiles {
			sdb.AddAddressToAccessList(addr)
		}
		for _, el := range txAccesses {
			sdb.AddAddressToAccessList(el.Address)
			for _, key := range el.StorageKeys {
				sdb.AddSlotToAccessList(el.Address, key)
			}
		}
		if rules.IsShanghai {
			sdb.AddAddressToAccessList(coinbase)
		}
	}
}

func (sdb *stateDB) AddressInAccessList(addr common.Address) bool {
	return sdb.alcalc || sdb.accessList.ContainsAddress(addr)
}
func (sdb *stateDB) SlotInAccessList(addr common.Address, slot ctypes.Hash) (addressOk bool, slotOk bool) {
	if sdb.alcalc {
		return true, true
	}
	return sdb.accessList.Contains(addr, slot)
}

// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (sdb *stateDB) AddAddressToAccessList(addr common.Address) {
	if sdb.accessList.AddAddress(addr) {
		sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.accessList.DeleteAddress(addr) }})
	}
}

// AddSlotToAccessList adds the given (address,slot) to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (sdb *stateDB) AddSlotToAccessList(addr common.Address, slot ctypes.Hash) {
	addrMod, slotMod := sdb.accessList.AddSlot(addr, slot)
	if addrMod {
		// In practice, this should not happen, since there is no way to enter the
		// scope of 'address' without having the 'address' become already added to
		// the access list (via call-variant, create, etc). Better safe than sorry,
		// though.
		sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.accessList.DeleteAddress(addr) }})
	}
	if slotMod {
		sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) { sdb.accessList.DeleteSlot(addr, slot) }})
	}
}
func (sdb *stateDB) RevertToSnapshot(snap int) {
	for i := len(sdb.journal) - 1; i >= snap; i-- {
		sdb.journal[i].revert(sdb)
	}
	sdb.journal = sdb.journal[:snap]
}
func (sdb *stateDB) Snapshot() int { return len(sdb.journal) }

func (sdb *stateDB) AddLog(log *types.Log) {

	sdb.journal = append(sdb.journal, journalEntry{nil, func(sdb *stateDB) {
        logs := sdb.logs[sdb.thash]
        if len(logs) > 0 {
            sdb.logs[sdb.thash] = logs[:len(logs)-1]
            sdb.logSize--
        }
    }})
	log.TxHash = sdb.thash
	log.TxIndex = uint(sdb.txIndex)
	log.Index = sdb.logSize
	sdb.logs[sdb.thash] = append(sdb.logs[sdb.thash], log)
	sdb.logSize++
}
func (sdb *stateDB) AddPreimage(ctypes.Hash, []byte) {
	// I doubt we'll ever support preimage tracking, but easier to leave a
	// placeholder than strip it out of the EVM
}

// func (sdb *stateDB) ForEachStorage(addr common.Address, func(ctypes.Hash, ctypes.Hash) bool) error {return nil}

// GetLogs returns the logs matching the specified transaction hash, and annotates
// them with the given blockNumber and blockHash.
func (s *stateDB) GetLogs(hash ctypes.Hash, blockNumber uint64, blockHash ctypes.Hash) []*types.Log {
	logs := s.logs[hash]
	for _, l := range logs {
		l.BlockNumber = blockNumber
		l.BlockHash = blockHash
	}
	return logs
}

// SetTxContext sets the current transaction hash and index which are
// used when the EVM emits new state logs. It should be invoked before
// transaction execution.
func (s *stateDB) SetTxContext(thash ctypes.Hash, ti int) {
	if s.logs == nil {
		s.logs = make(map[ctypes.Hash][]*types.Log)
	}
	s.thash = thash
	s.txIndex = ti
}
