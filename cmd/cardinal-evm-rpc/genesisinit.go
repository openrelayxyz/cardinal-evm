package main

import (
	"encoding/json"
	"fmt"
	log "github.com/inconshreveable/log15"
	"os"
	"math/big"
	"errors"

	"github.com/openrelayxyz/cardinal-storage/resolver"
	"github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/state"
)

type genesisBlock struct {
	// init.SetBlockData(r.Hash, r.ParentHash, r.Number, r.Weight.ToInt())
	Config params.ChainConfig      `json:"config"`
	Hash types.Hash                `json:"hash"`
	ParentHash types.Hash          `json:"parentHash"`
	Number hexutil.Uint64          `json:"number"`
	Weight hexutil.Uint64          `json:"difficulty"`
	Alloc      GenesisAlloc        `json:"alloc"`
}

type GenesisAlloc map[common.Address]GenesisAccount

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       hexutil.Bytes               `json:"code,omitempty"`
	Storage    map[types.Hash]types.Hash   `json:"storage,omitempty"`
	Balance    *hexutil.Big                `json:"balance"`
	Nonce      uint64                      `json:"nonce,omitempty"`
}


func genesisInit(dbpath, genesispath string, archival bool) error {
	gfile, err := os.Open(genesispath)
	decoder := json.NewDecoder(gfile)
	var gb genesisBlock
	if err := decoder.Decode(&gb); err != nil {
		return err
	}
	gfile.Close()
	init, err := resolver.ResolveInitializer(dbpath, archival)
	if err != nil { return err }
	if gb.Hash == (types.Hash{}) {
		return errors.New("hash must be set in genesis file")
	}
	var emptyAccount *state.Account
	init.SetBlockData(gb.Hash, gb.ParentHash, uint64(gb.Number), new(big.Int).SetInt64(int64(gb.Weight)))
	for addr, alloc := range gb.Alloc {
		if len(alloc.Code) != 0 {
			codeKey := fmt.Sprintf("c/%x/c/%x", gb.Config.ChainID, crypto.Keccak256(alloc.Code))
			init.AddData([]byte(codeKey), alloc.Code)
		}
		
		for storage, value := range alloc.Storage{
			key := fmt.Sprintf("c/%x/a/%x/s/%x", gb.Config.ChainID, crypto.Keccak256(addr[:]), crypto.Keccak256(storage[:]))
			init.AddData([]byte(key), value.Bytes())
		}
		acct := emptyAccount.Copy()
		acct.Nonce = alloc.Nonce
		acct.Balance.Set((*big.Int)(alloc.Balance))
		data, err := rlp.EncodeToBytes(acct)
		if err != nil { return err }
		key := fmt.Sprintf("c/%x/a/%x/d", gb.Config.ChainID, crypto.Keccak256(addr[:]))
		init.AddData([]byte(key), data)
		log.Debug("Added allocation", "addr", addr, "balance", acct.Balance, "key", key, "data", data)
	}
	init.Close()
	return nil
}