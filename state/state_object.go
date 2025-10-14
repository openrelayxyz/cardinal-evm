package state

import (
	"bytes"
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types"
	"math/big"
)

type Storage map[types.Hash]types.Hash

func (s Storage) Copy() Storage {
	result := make(Storage)
	for k, v := range s {
		result[k] = v
	}
	return result
}

type codeEntry struct {
	code []byte
	hash types.Hash
}

func (c *codeEntry) getHash() types.Hash {
	if c.hash != (types.Hash{}) {
		c.hash = crypto.Keccak256Hash(c.code)
	}
	return c.hash
}

type stateObject struct {
	address      common.Address
	account      *Account
	balanceDelta *big.Int
	code         *codeEntry
	dirty        Storage
	clean        Storage
	created      bool
	suicided     bool
	deleted      bool
	nonce        *uint64
	fakeBalance  *big.Int
}

func (s *stateObject) kv(chainid int64) []storage.KeyValue {
	if s.deleted || s.suicided {
		return []storage.KeyValue{}
	}
	result := make([]storage.KeyValue, 0, len(s.dirty)+len(s.clean)+2)
	acct := s.account.Copy()
	if s.code != nil {
		copy(acct.CodeHash, s.code.getHash().Bytes())
	}
	acct.Balance = s.getBalance()
	acctRLP, _ := rlp.EncodeToBytes(s.account)
	result = append(result, storage.KeyValue{schema.AccountData(chainid, s.address.Bytes()), acctRLP})
	if s.code != nil {
		result = append(result, storage.KeyValue{schema.AccountCode(chainid, s.code.hash.Bytes()), s.code.code})
	}
	for k, v := range s.dirty {
		result = append(result, storage.KeyValue{schema.AccountStorage(chainid, s.address.Bytes(), k.Bytes()), v.Bytes()})
	}
	for k, v := range s.clean {
		if _, ok := s.dirty[k]; !ok {
			result = append(result, storage.KeyValue{schema.AccountStorage(chainid, s.address.Bytes(), k.Bytes()), v.Bytes()})
		}
	}
	return result
}

func (s *stateObject) equal(b *stateObject) bool {
	if s.address != b.address || s.balanceDelta.Cmp(b.balanceDelta) != 0 || len(s.clean) != len(b.clean) || s.suicided != b.suicided || s.deleted != b.deleted || *s.nonce != *b.nonce {
		return false
	}
	if s.account != nil && b.account != nil {
		if s.account.Nonce != b.account.Nonce {
			return false
		}
		if !bytes.Equal(s.account.Root, b.account.Root) {
			return false
		}
		if !bytes.Equal(s.account.CodeHash, b.account.CodeHash) {
			return false
		}
		if s.getBalance().Cmp(b.getBalance()) != 0 {
			return false
		}
	}
	if s.code != nil && b.code != nil {
		if s.code.getHash() != b.code.getHash() {
			return false
		}
	}
	for k, v := range s.clean {
		if b.clean[k] != v {
			return false
		}
	}
	for k, v := range s.dirty {
		if b.dirty[k] != v {
			return false
		}
	}
	return true
}

func (s *stateObject) copy() *stateObject {
	state := &stateObject{
		address:  s.address,
		account:  s.account,
		code:     s.code,
		dirty:    s.dirty.Copy(),
		clean:    s.clean.Copy(),
		created:  s.created,
		suicided: s.suicided,
		deleted:  s.deleted,
	}
	if s.balanceDelta != nil {
		state.balanceDelta = new(big.Int).Set(s.balanceDelta)
	}
	if s.nonce != nil {
		state.nonce = &(*s.nonce)
	}
	if s.fakeBalance != nil {
		state.fakeBalance = new(big.Int).Set(s.fakeBalance)
	}
	return state
}

func (s *stateObject) finalise() {
	for k, v := range s.dirty {
		s.clean[k] = v
	}
	if s.suicided {
		s.deleted = true
	}
}

func (s *stateObject) loadAccount(tx storage.Transaction, chainid int64) bool {
	if s.deleted {
		return false
	}
	if s.account == nil {
		err := tx.ZeroCopyGet(schema.AccountData(chainid, s.address.Bytes()), func(acctData []byte) error {
			account, err := FullAccount(acctData)
			if err != nil {
				return err
			}
			s.account = &account
			return nil
		})
		if err == storage.ErrNotFound {
			// TODO: Some discrepancy between here and badgerdb storage
			log.Debug("Account not found", "key", string(schema.AccountData(chainid, s.address.Bytes())))
			return false
		}
		if err != nil {
			log.Error("Error parsing account", "addr", s.address, "err", err)
			return false
		}
		return true
	}
	return true
}
func (s *stateObject) loadCode(tx storage.Transaction, chainid int64) bool {
	if s.deleted {
		return false
	}
	if s.code != nil {
		return true
	}
	if !s.loadAccount(tx, chainid) || (s.account != nil && bytes.Equal(s.account.CodeHash, emptyCode.Bytes())) {
		s.code = &codeEntry{
			code: []byte{},
			hash: emptyCode,
		}
		return true
	}
	code, _ := tx.Get(schema.AccountCode(chainid, s.account.CodeHash))
	s.code = &codeEntry{
		code: code,
		hash: types.BytesToHash(s.account.CodeHash),
	}
	return true
}

func (s *stateObject) subBalance(amount *big.Int) journalEntry {
	if s.balanceDelta == nil {
		s.balanceDelta = new(big.Int).Neg(amount)
		return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).balanceDelta = nil }}
	}
	s.balanceDelta.Sub(s.balanceDelta, amount)
	return journalEntry{&s.address, func(sdb *stateDB) {
		rs := sdb.getAccount(s.address)
		rs.balanceDelta.Add(rs.balanceDelta, amount)
	}}
}
func (s *stateObject) addBalance(amount *big.Int) journalEntry {
	if s.balanceDelta == nil {
		s.balanceDelta = new(big.Int).Set(amount)
		return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).balanceDelta = nil }}
	}
	s.balanceDelta.Add(s.balanceDelta, amount)
	return journalEntry{&s.address, func(sdb *stateDB) {
		rs := sdb.getAccount(s.address)
		rs.balanceDelta.Sub(rs.balanceDelta, amount)
	}}
}

func (s *stateObject) getBalance() *big.Int {
	var balance *big.Int
	delta := s.balanceDelta
	if delta == nil {
		delta = new(big.Int)
	}
	if s.fakeBalance != nil {
		balance = s.fakeBalance
	} else if s.account != nil {
		balance = s.account.Balance
	}
	if balance == nil {
		balance = new(big.Int)
	}
	return new(big.Int).Add(delta, balance)
}
func (s *stateObject) setBalance(balance *big.Int) journalEntry {
	old := s.fakeBalance
	s.fakeBalance = balance
	return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).fakeBalance = old }}
}

func (s *stateObject) getNonce(tx storage.Transaction, chainid int64) uint64 {
	if s.nonce != nil {
		return *s.nonce
	}
	if !s.loadAccount(tx, chainid) {
		return 0
	}
	return s.account.Nonce
}
func (s *stateObject) setNonce(nonce uint64) journalEntry {
	oldNonce := s.nonce
	s.nonce = &nonce
	return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).nonce = oldNonce }}
}

func (s *stateObject) getCodeHash(tx storage.Transaction, chainid int64) types.Hash {
	if s.account != nil {
		return types.BytesToHash(s.account.CodeHash)
	}
	if !s.loadCode(tx, chainid) {
		return emptyCode
	}
	return s.code.getHash()
}

func (s *stateObject) getCode(tx storage.Transaction, chainid int64) []byte {
	if !s.loadCode(tx, chainid) {
		return []byte{}
	}
	return s.code.code
}

func (s *stateObject) setCode(code []byte) journalEntry {
	old := s.code
	s.code = &codeEntry{
		code: code,
		hash: crypto.Keccak256Hash(code),
	}
	return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).code = old }}
}
func (s *stateObject) getCommittedState(tx storage.Transaction, chainid int64, storage types.Hash) types.Hash {
	if data, ok := s.clean[storage]; ok {
		return data
	}
	tx.ZeroCopyGet(schema.AccountStorage(chainid, s.address.Bytes(), storage.Bytes()), func(data []byte) error {
		// BytesToHash performs a copy operation, so this may be more efficient
		// than using Get()
		var value types.Hash
		if len(data) > 0 {
			_, content, _, _ := rlp.Split(data)
			value.SetBytes(content)
		}
		s.clean[storage] = value
		return nil
	})
	return s.clean[storage]
}

func (s *stateObject) getState(tx storage.Transaction, chainid int64, storage types.Hash) types.Hash {
	if data, ok := s.dirty[storage]; ok {
		return data
	}
	return s.getCommittedState(tx, chainid, storage)
}
func (s *stateObject) setState(storage, data types.Hash) journalEntry {
	if old, ok := s.dirty[storage]; ok {
		s.dirty[storage] = data
		return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).dirty[storage] = old }}
	}
	s.dirty[storage] = data
	return journalEntry{&s.address, func(sdb *stateDB) { delete(sdb.getAccount(s.address).dirty, storage) }}
}
func (s *stateObject) setStorage(storage map[types.Hash]types.Hash) journalEntry {
	old := s.dirty
	s.dirty = storage
	return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).dirty = old }}
}

func (s *stateObject) suicide() (bool, *journalEntry) {
	if s.suicided || s.deleted {
		return false, nil
	}
	s.suicided = true
	return true, &journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).suicided = false }}
}

func (s *stateObject) empty(tx storage.Transaction, chainid int64) bool {
	return s.getNonce(tx, chainid) == 0 && s.getBalance().Sign() == 0 && s.getCodeHash(tx, chainid) == emptyCode
}
