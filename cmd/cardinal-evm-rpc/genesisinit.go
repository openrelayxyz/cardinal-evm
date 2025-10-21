package main

import (
	"encoding/json"
	"fmt"
	log "github.com/inconshreveable/log15"
	"os"
	"math/big"
	"errors"

	"github.com/openrelayxyz/cardinal-storage/resolver"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-evm/types"
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
	Hash ctypes.Hash                `json:"hash"`
	ParentHash ctypes.Hash          `json:"parentHash"`
	Number hexutil.Uint64          `json:"number"`
	Weight hexutil.Uint64          `json:"difficulty"`
	Alloc      GenesisAlloc        `json:"alloc"`
	GasLimit   hexutil.Uint64     `json:"gasLimit"`
	Coinbase   common.Address     `json:"coinbase"`
	Timestamp  uint64     		`json:"timestamp"`
	ExtraData  hexutil.Bytes      `json:"extraData"`
	Nonce      hexutil.Uint64     `json:"nonce"`
	MixHash    ctypes.Hash         `json:"mixhash"`
}

type GenesisAlloc map[string]GenesisAccount

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       hexutil.Bytes               `json:"code,omitempty"`
	Storage    map[ctypes.Hash]ctypes.Hash   `json:"storage,omitempty"`
	Balance    *hexutil.Big                `json:"balance"`
	Nonce      hexutil.Uint64              `json:"nonce,omitempty"`
}


func genesisInit(dbpath, genesispath string, archival bool) error {
	gfile, err := os.Open(genesispath)      
	if err != nil {return err}
	decoder := json.NewDecoder(gfile)
	var gb genesisBlock
	if err := decoder.Decode(&gb); err != nil {
		return err
	}
	gfile.Close()
	init, err := resolver.ResolveInitializer(dbpath, archival)
	if err != nil { return err }
	if gb.Hash == (ctypes.Hash{}) {
		return errors.New("hash must be set in genesis file")
	}
	var emptyAccount *state.Account
	init.SetBlockData(gb.Hash, gb.ParentHash, uint64(gb.Number), new(big.Int).SetInt64(int64(gb.Weight)))
	header:= &types.Header{
		Number: big.NewInt(int64(gb.Number)),
		ParentHash: ctypes.Hash(gb.ParentHash),
		Difficulty: big.NewInt(int64(gb.Weight)),
		GasLimit: uint64(gb.GasLimit),
		Time:  uint64(gb.Timestamp),
		Extra: gb.ExtraData,
		MixDigest: ctypes.Hash(gb.MixHash),
		Nonce: types.EncodeNonce(uint64(gb.Nonce)),
	}
	rawHeader,_ := rlp.EncodeToBytes(header)
	headerKey := fmt.Sprintf("c/%x/b/%x/h", gb.Config.ChainID, gb.Hash.Bytes())
	init.AddData([]byte(headerKey), rawHeader)
	
	for addrString, alloc := range gb.Alloc {
		addr := common.HexToAddress(addrString)
		if len(alloc.Code) != 0 {
			codeKey := fmt.Sprintf("c/%x/c/%x", gb.Config.ChainID, crypto.Keccak256(alloc.Code))
			init.AddData([]byte(codeKey), alloc.Code)
		}
		
		for storage, value := range alloc.Storage{
			key := fmt.Sprintf("c/%x/a/%x/s/%x", gb.Config.ChainID, crypto.Keccak256(addr[:]), crypto.Keccak256(storage[:]))
			init.AddData([]byte(key), value.Bytes())
		}
		acct := emptyAccount.Copy()
		acct.Nonce = uint64(alloc.Nonce)
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