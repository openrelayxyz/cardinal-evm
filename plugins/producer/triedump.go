package main

import (
	"bytes"
	"context"
	"strconv"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted/types"
	"github.com/openrelayxyz/plugeth-utils/restricted/rlp"
)

func trieDump (ctx core.Context, args []string) error {
	log.Info("Starting trie dump")
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
		log.Error("error getting trie", "block", i)
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
		destructedAccounts := map[string]struct{}{}


		COMPARE_NODES:
		for {
			switch compareNodes(a, b) {
			case -1:
				// Node exists in lastTrie but not currentTrie
				// This is a deletion
				if a.Leaf() {
					destructedAccounts[string(a.LeafKey())] = struct{}{}
				}

				// b jumped past a; advance a
				if !a.Next(true) {
					break COMPARE_NODES
				}
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
		for k := range destructedAccounts {
			log.Info("Destructed", "account", []byte(k))
		}
		for k, v := range alteredAccounts {
			log.Info("Altered", "account", []byte(k), "data", v)
		}
	}
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
	if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && b.Leaf() {
		return bytes.Compare(a.LeafBlob(), b.LeafBlob())
	}
	return 0
}