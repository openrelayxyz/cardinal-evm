package main

import (
	"fmt"
	"math/big"
	"encoding/json"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted"
	"github.com/openrelayxyz/plugeth-utils/restricted/rlp"
	"github.com/openrelayxyz/plugeth-utils/restricted/types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
	"time"
	"strconv"
)

var (
	log core.Logger
	postMerge bool
	backend restricted.Backend
	stack core.Node
	chainid int64
	client core.Client
	targetVerbosity int
)

func Initialize(ctx core.Context, loader core.PluginLoader, logger core.Logger) {
	log = logger
	log.Info("Cardinal EVM polygon plugin initializing")
	var err error
	targetVerbosity, err = strconv.Atoi(ctx.String("verbosity"))
	if err != nil {
		targetVerbosity = 3
	}
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
	var x interface{}
	client.Call(&x, "debug_verbosity", targetVerbosity + 1)
	time.AfterFunc(30 * time.Second, func() {
		client.Call(&x, "debug_verbosity", targetVerbosity + 1)
	})
	log.Info("Cardinal EVM resetting log level")
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

	var sprint int64
	if number < 38189056 {
		sprint = 64 
	} else {
		sprint = 16
	}

	if number % sprint == 0 {
		var borsnap json.RawMessage
		if err := client.Call(&borsnap, "bor_getSnapshot", hexutil.Uint64(number)); err != nil {
			log.Error("Error retrieving bor snapshot on block %v", number)
		}
		updates[fmt.Sprintf("c/%x/b/%x/bs", uint64(chainid), hash.Bytes())] = borsnap
	}



	var receipt types.Receipt
	if err := client.Call(&receipt, "eth_getBorBlockReceipt", hash); err != nil {
		log.Debug("No bor receipt", "blockno", number, "hash", hash, "err", err)
		return
	}
	updates[fmt.Sprintf("c/%x/b/%x/br/%x", uint64(chainid), hash.Bytes(), receipt.TransactionIndex)] = receipt.Bloom.Bytes()
	for _, logRecord := range receipt.Logs {
		updates[fmt.Sprintf("c/%x/b/%x/bl/%x/%x", chainid, hash.Bytes(), receipt.TransactionIndex, logRecord.Index)], _ = rlp.EncodeToBytes(logRecord)
	}
}
