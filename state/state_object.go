package state

import (
  "bytes"
  "math/big"
  "github.com/openrelayxyz/cardinal-types"
  "github.com/openrelayxyz/cardinal-storage"
  "github.com/openrelayxyz/cardinal-evm/common"
  "github.com/openrelayxyz/cardinal-evm/crypto"
  "github.com/openrelayxyz/cardinal-evm/schema"
  log "github.com/inconshreveable/log15"
)

type Storage map[types.Hash]types.Hash

func (s Storage) Copy() Storage {
  result := make(Storage)
  for k, v := range s {
    result[k] = v
  }
  return result
}

type codeEntry struct{
  code []byte
  hash types.Hash
}

func (c *codeEntry) getHash() types.Hash {
  if c.hash != (types.Hash{}) {
    c.hash = crypto.Keccak256Hash(c.code)
  }
  return c.hash
}

type stateObject struct{
  address      common.Address
  account      *Account
  balanceDelta *big.Int
  code         *codeEntry
  dirty        Storage
  clean        Storage
  suicided     bool
  deleted      bool
  nonce        *uint64
}

func (s *stateObject) copy() *stateObject {
  return &stateObject{
    address: s.address,
    account: s.account,
    balanceDelta: new(big.Int).Set(s.balanceDelta),
    code: s.code,
    dirty: s.dirty.Copy(),
    clean: s.clean.Copy(),
    suicided: s.suicided,
    deleted: s.deleted,
    nonce: &(*s.nonce),
  }
}

func (s *stateObject) finalise() {
  for k, v := range s.dirty{
    s.clean[k] = v
  }
  if s.suicided { s.deleted = true }
}

func (s *stateObject) loadAccount(tx storage.Transaction, chainid int64) bool {
  if s.deleted { return false }
  if s.account == nil {
    acctData, err := tx.Get(schema.AccountData(chainid, s.address.Bytes()))
    if err == storage.ErrNotFound { return false }
    account, err := FullAccount(acctData)
    s.account = &account
    if err != nil {
      log.Error("Error parsing account", "addr", s.address, "err", err)
      return false
    }
    return true
  }
  return true
}
func (s *stateObject) loadCode(tx storage.Transaction, chainid int64) bool {
  if s.deleted { return false }
  if s.code != nil { return true }
  if s.account != nil && bytes.Equal(s.account.CodeHash, emptyCode.Bytes()) {
    s.code = &codeEntry{
      code: []byte{},
      hash: emptyCode,
    }
    return true
  }
  code, _ := tx.Get(schema.AccountCode(chainid, s.address.Bytes()))
  ce := &codeEntry{
    code: code,
  }
  if s.account != nil {
    ce.hash = types.BytesToHash(s.account.CodeHash)
  }
  s.code = ce
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
  if s.balanceDelta == nil { return s.account.Balance }
  return new(big.Int).Add(s.account.Balance, s.balanceDelta)
}

func (s *stateObject) getNonce(tx storage.Transaction, chainid int64) uint64 {
  if s.nonce != nil { return *s.nonce }
  if !s.loadAccount(tx, chainid) { return 0 }
  return s.account.Nonce
}
func (s *stateObject) setNonce(nonce uint64) journalEntry {
  oldNonce := s.nonce
  s.nonce = &nonce
  return journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).nonce = oldNonce }}
}

func (s *stateObject) getCodeHash(tx storage.Transaction, chainid int64) types.Hash {
  if s.account != nil { return types.BytesToHash(s.account.CodeHash) }
  if !s.loadCode(tx, chainid) { return emptyCode }
  return s.code.getHash()
}

func (s *stateObject) getCode(tx storage.Transaction, chainid int64) []byte {
  if !s.loadCode(tx, chainid) { return []byte{} }
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
  if data, ok := s.clean[storage]; ok { return data }
  data, _ := tx.Get(schema.AccountStorage(chainid, s.address.Bytes(), storage.Bytes()))
  s.clean[storage] = types.BytesToHash(data)
  return s.clean[storage]
}

func (s *stateObject) getState(tx storage.Transaction, chainid int64, storage types.Hash) types.Hash {
  if data, ok := s.dirty[storage]; ok { return data }
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

func (s *stateObject) suicide() (bool, *journalEntry) {
  if s.suicided || s.deleted { return false, nil }
  s.suicided = true
  return true, &journalEntry{&s.address, func(sdb *stateDB) { sdb.getAccount(s.address).suicided = false }}
}

func (s *stateObject) empty(tx storage.Transaction, chainid int64) bool {
  return s.getNonce(tx, chainid) == 0 && s.getBalance().Sign() == 0 && s.getCodeHash(tx, chainid) == emptyCode
}
