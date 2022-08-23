package main

import (
	"fmt"
	"math/big"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted"
	"github.com/openrelayxyz/plugeth-utils/restricted/rlp"
	"github.com/openrelayxyz/plugeth-utils/restricted/types"
	"gopkg.in/urfave/cli.v1"
)

var (
	log core.Logger
	postMerge bool
	backend restricted.Backend
	stack core.Node
	chainid int64
	client core.Client
)

func Initialize(ctx *cli.Context, loader core.PluginLoader, logger core.Logger) {
	log = logger
	log.Info("Cardinal EVM polygon plugin initializing")
}

func InitializeNode(s core.Node, b restricted.Backend) {
	backend = b
	stack = s
	config := b.ChainConfig()
	chainid = config.ChainID.Int64()
	var err error
	client, err = stack.Attach()
	if err != nil {
		log.Warn("Failed to initialize RPC client, cannot process block")
	}
}

func UpdateStreamsSchema(schema map[string]string) {
	schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/br/", chainid)] = schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/r/", chainid)]
	schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/bl/", chainid)] = schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/l/", chainid)]
	schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/bs", chainid)] = schema[fmt.Sprintf("c/%x/b/[0-9a-z]+/h", chainid)]
}

func CardinalAddBlockHook(number int64, hash, parent ctypes.Hash, weight *big.Int, updates map[string][]byte, deletes map[string]struct{}) {
	if client == nil {
		log.Warn("Failed to initialize RPC client, cannot process block")
		return
	}
	var receipt types.Receipt
	err := client.Call(&receipt, "eth_getBorBlockReceipt", hash)
	if err != nil {
		log.Info("No bor receipt", "blockno", number, "hash", hash, "err", err)
		return
	}
	borsnap, err := backend.ChainDb().Get(append([]byte("bor-"), hash[:]...))
	updates[fmt.Sprintf("c/%x/b/%x/bs", uint64(chainid), hash.Bytes())] = borsnap
	updates[fmt.Sprintf("c/%x/b/%x/br/%x", uint64(chainid), hash.Bytes(), receipt.TransactionIndex)] = receipt.Bloom.Bytes()
	for _, logRecord := range receipt.Logs {
		updates[fmt.Sprintf("c/%x/b/%x/bl/%x/%x", chainid, hash.Bytes(), receipt.TransactionIndex, logRecord.Index)], _ = rlp.EncodeToBytes(logRecord)
	}
}
