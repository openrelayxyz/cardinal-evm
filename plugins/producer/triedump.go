package main

import (
	// "fmt"
	"bytes"
	"context"
	"strconv"
	"github.com/hashicorp/golang-lru"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted/types"
	"github.com/openrelayxyz/plugeth-utils/restricted/rlp"
)

var (
	headerCache *lru.Cache
)

func traverseIterators(a, b core.NodeIterator, add, delete func(k, v []byte), alter func(k1, v1, k2, v2 []byte)) {
	// Advance both iterators initially
	hasA := a.Next(true)
	hasB := b.Next(true)
	counter := 0

	for hasA && hasB {
		counter++
		switch compareNodes(a, b) {
		case -1: // a is behind b
			if a.Leaf() {
				delete(a.LeafKey(), a.LeafBlob())
			}
			hasA = a.Next(true) // advance only a
		case 1: // a is ahead of b
			if b.Leaf() {
				add(b.LeafKey(), b.LeafBlob())
			}
			hasB = b.Next(true) // advance only d
		case 0: // nodes are equal
			if a.Leaf() && b.Leaf() {
				if !bytes.Equal(a.LeafBlob(), b.LeafBlob()) {
					alter(a.LeafKey(), a.LeafBlob(), b.LeafKey(), b.LeafBlob())
					// add(b.LeafKey(), b.LeafBlob())
				}
			}
			descend := a.Hash() != b.Hash()
			if !descend && a.Hash() == (core.Hash{}) {
				descend = true
			}
			hasA = a.Next(descend)
			hasB = b.Next(descend) // advance both iterators
		}
	}

	// Handle remaining nodes in A
	for hasA {
		counter++
		if a.Leaf() {
			delete(a.LeafKey(), a.LeafBlob())
		}
		hasA = a.Next(true)
	}

	// Handle remaining nodes in B
	for hasB {
		counter++
		if b.Leaf() {
			add(b.LeafKey(), b.LeafBlob())
		}
		hasB = b.Next(true)
	}
	log.Info("Processed tries.", "nodes", counter)
}

func stateTrieUpdatesByNumber(i int64) (map[core.Hash]struct{}, map[core.Hash][]byte, map[core.Hash]map[core.Hash][]byte, map[core.Hash][]byte, error) {
	if headerCache == nil {
		headerCache, _ = lru.New(64)
	}
	destructs := make(map[core.Hash]struct{})
	accounts := make(map[core.Hash][]byte)
	storage := make(map[core.Hash]map[core.Hash][]byte)
	code := make(map[core.Hash][]byte)
	db := backend.ChainDb()

	var lastHeader, header *types.Header
	var err error

	if v, ok := headerCache.Get(i-1); ok {
		lastHeader = v.(*types.Header)
	} else {
		lastHeaderBytes, err := backend.HeaderByNumber(context.Background(), i-1)
		if err != nil {
			log.Warn("Error getting starting header")
			return nil, nil, nil, nil, err
		}
		lastHeader = &types.Header{}
		if err := rlp.DecodeBytes(lastHeaderBytes, lastHeader); err != nil {
			log.Error("Error decoding header", "block", startBlock, "err", err)
			return nil, nil, nil, nil, err
		}
		headerCache.Add(i-1, lastHeader)
	}
	var lastTrie core.Trie
	lastTrie, err = backend.GetTrie(lastHeader.Root)
	if err != nil {
		log.Error("error getting trie", "block", startBlock)
		return nil, nil, nil, nil, err
	}

	if v, ok := headerCache.Get(i); ok {
		header = v.(*types.Header)
	} else {
		header = &types.Header{}
		headerBytes, err := backend.HeaderByNumber(context.Background(), i)
		if err != nil {
			log.Error("")
			return nil, nil, nil, nil, err
		}
		if err := rlp.DecodeBytes(headerBytes, &header); err != nil {
			log.Error("Error decoding header", "block", i, "err", err)
			return nil, nil, nil, nil, err
		}
		headerCache.Add(i, header)
	}
	currentTrie, err := backend.GetTrie(header.Root)
	if err != nil {
		log.Error("Error getting last trie")
		return nil, nil, nil, nil, err
	}
	a := lastTrie.NodeIterator(nil)
	b := currentTrie.NodeIterator(nil)
	alteredAccounts := map[string]acct{}
	oldAccounts := map[string]acct{}
	codeChanges := map[core.Hash]struct{}{}


	traverseIterators(a, b, func(k, v []byte){
		// Added accounts
		account, err := fullAccount(v)
		if err != nil {
			log.Warn("Found invalid account in acount trie")
			return
		}
		alteredAccounts[string(k)] = account
	}, 
	func(k, v []byte) {
		// Deleted accounts
		account, err := fullAccount(v)
		if err != nil {
			log.Warn("Found invalid account in acount trie")
			return
		}
		oldAccounts[string(k)] = account
	},
	func(oldk, oldv, newk, newv []byte) {
		account, err := fullAccount(oldv)
		if err != nil {
			log.Warn("Found invalid account in acount trie")
			return
		}
		oldAccounts[string(oldk)] = account
		account, err = fullAccount(newv)
		if err != nil {
			log.Warn("Found invalid account in acount trie")
			return
		}
		alteredAccounts[string(newk)] = account
	})
	storageChanges := map[string]map[string][]byte{}
	// TODO: Iteration of the altered accounts could potentially be parallelized
	for k, acct := range alteredAccounts {
		var oldStorageTrie core.Trie
		if oldAcct, ok := oldAccounts[k]; ok {
			delete(oldAccounts, k)
			if !bytes.Equal(oldAcct.CodeHash, acct.CodeHash) {
				codeChanges[core.BytesToHash(acct.CodeHash)] = struct{}{}
			}
			if bytes.Equal(acct.Root, oldAcct.Root) {
				// Storage didn't change
				continue
			}
			oldStorageTrie, err = backend.GetTrie(core.BytesToHash(oldAcct.Root))
		} else {
			if !bytes.Equal(acct.CodeHash, emptyCode) {
				codeChanges[core.BytesToHash(acct.CodeHash)] = struct{}{}
			}
			if bytes.Equal(acct.Root, emptyRoot) {
				// Storage for new account is empty
				continue
			}
			oldStorageTrie, err = backend.GetTrie(core.BytesToHash(emptyRoot))
		}
		storageTrie, err := backend.GetTrie(core.BytesToHash(acct.Root))
		if err != nil {
			log.Error("error getting trie for account", "acct", k)
			return nil, nil, nil, nil, err
		}
		c := oldStorageTrie.NodeIterator(nil)
		d := storageTrie.NodeIterator(nil)
		storageChanges[k] = map[string][]byte{}
		traverseIterators(c, d, func(key, v []byte) {
			// Add Storage
			storageChanges[k][string(key)] = v

		}, func(key, v []byte) {
			// Delete Storage
			if _, ok := storageChanges[k][string(key)]; !ok {
				storageChanges[k][string(key)] = []byte{}
			}
		}, func(oldk, oldv, newk, newv []byte) {
			storageChanges[k][string(newk)] = newv
		})
	}
	for k := range oldAccounts {
		destructs[core.BytesToHash([]byte(k))] = struct{}{}
	}
	for k, v := range alteredAccounts {
		accounts[core.BytesToHash([]byte(k))], err = v.slimRLP()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		storage[core.BytesToHash([]byte(k))] = map[core.Hash][]byte{}
		for sk, sv := range storageChanges[k] {
			storage[core.BytesToHash([]byte(k))][core.BytesToHash([]byte(sk))] = sv
		}
	}
	for codeHash := range codeChanges {
		if !bytes.Equal(codeHash.Bytes(), emptyCode) {
			c, _ := db.Get(append(codePrefix, codeHash.Bytes()...))
			if len(c) == 0 {
				c, err = db.Get(codeHash.Bytes())
				if err != nil {
					return nil, nil, nil, nil, err
				}
			}
			code[core.BytesToHash(codeHash.Bytes())] = c
		}
	}
	return destructs, accounts, storage, code, nil
}


func trieDump (ctx core.Context, args []string) error {
	log.Info("Starting trie dump")
	// chainConfig := backend.ChainConfig()
	header := &types.Header{}
	rlp.DecodeBytes(backend.CurrentHeader(), &header)
	startBlock := int64(0)
	endBlock := header.Number.Int64()
	if len(args) > 0 {
		s, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		startBlock = int64(s)
	}
	if len(args) > 1 {
		e, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		endBlock = int64(e)
	}
	lastHeaderBytes, err := backend.HeaderByNumber(context.Background(), startBlock)
	if err != nil {
		log.Warn("Error getting starting header")
		return err
	}
	lastHeader := &types.Header{}
	if err := rlp.DecodeBytes(lastHeaderBytes, lastHeader); err != nil {
		log.Error("Error decoding header", "block", startBlock, "err", err)
		return err
	}
	var lastTrie core.Trie
	lastTrie, err = backend.GetTrie(lastHeader.Root)
	if err != nil {
		log.Error("error getting trie", "block", startBlock)
	}
	for i := startBlock+1; i <= endBlock; i++ {
		headerBytes, err := backend.HeaderByNumber(context.Background(), i)
		if err != nil {
			log.Error("")
			return err
		}
		if err := rlp.DecodeBytes(headerBytes, &header); err != nil {
			log.Error("Error decoding header", "block", i, "err", err)
			return err
		}
		currentTrie, err := backend.GetTrie(header.Root)
		if err != nil {
			log.Error("Error getting last trie")
			return err
		}
		a := lastTrie.NodeIterator(nil)
		b := currentTrie.NodeIterator(nil)

		alteredAccounts := map[string]acct{}
		oldAccounts := map[string]acct{}


		COMPARE_NODES:
		for {
			switch compareNodes(a, b) {
			case -1:
				// Node exists in lastTrie but not currentTrie
				// This is a deletion
				if a.Leaf() {
					account, err := fullAccount(a.LeafBlob())
					if err != nil {
						log.Warn("Found invalid account in acount trie")
						continue
					}
					oldAccounts[string(a.LeafKey())] = account
				}

				// b jumped past a; advance a
				a.Next(true)
			case 1:
				// Node exists in currentTrie but not lastTrie
				// This is an addition

				if b.Leaf() {
					account, err := fullAccount(b.LeafBlob())
					if err != nil {
						log.Warn("Found invalid account in acount trie")
						continue
					}
					alteredAccounts[string(b.LeafKey())] = account
				}

				if !b.Next(true) {
					break COMPARE_NODES
				}

			case 0:
				// a and b are identical; skip this whole subtree if the nodes have hashes
				hasHash := a.Hash() == core.Hash{}
				if !b.Next(hasHash) {
					break COMPARE_NODES
				}
				if !a.Next(hasHash) {
					break COMPARE_NODES
				}
			}
		}
		storageChanges := map[string]map[string][]byte{}
		for k, acct := range alteredAccounts {
			var oldStorageTrie core.Trie
			if oldAcct, ok := oldAccounts[k]; ok {
				delete(oldAccounts, k)
				if bytes.Equal(acct.Root, oldAcct.Root) {
					// Storage didn't change
					continue
				}
				oldStorageTrie, err = backend.GetTrie(core.BytesToHash(oldAcct.Root))
			} else {
				oldStorageTrie, err = backend.GetTrie(core.BytesToHash(emptyRoot))
			}
			if bytes.Equal(acct.Root, emptyRoot) {
				// Empty trie, nothing to see here
				continue
			}
			storageTrie, err := backend.GetTrie(core.BytesToHash(acct.Root))
			if err != nil {
				log.Error("error getting trie for account", "acct", k)
				return err
			}
			c := oldStorageTrie.NodeIterator(nil)
			d := storageTrie.NodeIterator(nil)
			storageChanges[k] = map[string][]byte{}
			COMPARE_STORAGE:
			for {
				switch compareNodes(c, d) {
				case -1:
					// Node exists in lastTrie but not currentTrie
					// This is a deletion
					if c.Leaf() {
						storageChanges[k][string(c.LeafKey())] = []byte{}
						// storageChanges[fmt.Sprintf("c/%x/c/%x", chainConfig.ChainID, []byte(k), c.LeafKey())] = []byte{}
					}

					// c jumped past d; advance d
					c.Next(true)
				case 1:
					// Node exists in currentTrie but not lastTrie
					// This is an addition

					if d.Leaf() {
						storageChanges[k][string(d.LeafKey())] = d.LeafBlob()
					}

					if !d.Next(true) {
						break COMPARE_STORAGE
					}

				case 0:
					// a and b are identical; skip this whole subtree if the nodes have hashes
					hasHash := c.Hash() == core.Hash{}
					if !d.Next(hasHash) {
						break COMPARE_STORAGE
					}
					if !c.Next(hasHash) {
						break COMPARE_STORAGE
					}
				}
			}

		}
		for k := range oldAccounts {
			log.Info("Destructed", "account", []byte(k))
		}
		for k, v := range alteredAccounts {
			log.Info("Altered", "block", i, "account", []byte(k), "data", v)
			for sk, sv := range storageChanges[k] {
				log.Info("Storage key", "key", sk, "val", sv)
			}
		}
		lastTrie = currentTrie
	}
	return nil
}


func compareNodes(a, b core.NodeIterator) int {
	if cmp := bytes.Compare(a.Path(), b.Path()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && !b.Leaf() {
		return -1
	} else if b.Leaf() && !a.Leaf() {
		return 1
	}
	// if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
	// 	return cmp
	// }
	// if a.Leaf() && b.Leaf() {
	// 	return bytes.Compare(a.LeafBlob(), b.LeafBlob())
	// }
	return 0
}