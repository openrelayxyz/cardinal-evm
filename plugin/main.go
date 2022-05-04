package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"net"
	"time"
	"github.com/Shopify/sarama"
	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/metrics"
	"github.com/openrelayxyz/cardinal-streams/transports"
	"github.com/openrelayxyz/plugeth-utils/core"
	"github.com/openrelayxyz/plugeth-utils/restricted"
	"github.com/openrelayxyz/plugeth-utils/restricted/rlp"
	"github.com/openrelayxyz/plugeth-utils/restricted/types"
	"github.com/openrelayxyz/plugeth-utils/restricted/params"
	"github.com/savaki/cloudmetrics"
	"github.com/pubnub/go-metrics-statsd"
	"gopkg.in/urfave/cli.v1"
	"strings"
	"sync"
)

var (
	log core.Logger
	ready sync.WaitGroup
	backend restricted.Backend
	config *params.ChainConfig
	chainid int64
	producer transports.Producer
	startBlock uint64
	pendingReorgs map[core.Hash]func()
	gethHeightGauge = metrics.NewMajorGauge("/geth/height")
	gethPeersGauge = metrics.NewMajorGauge("/geth/peers")
	masterHeightGauge = metrics.NewMajorGauge("/master/height")

	Flags = *flag.NewFlagSet("cardinal-plugin", flag.ContinueOnError)
	txPoolTopic = Flags.String("cardinal.txpool.topic", "", "Topic for mempool transaction data")
	brokerURL = Flags.String("cardinal.broker.url", "", "URL of the Cardinal Broker")
	defaultTopic = Flags.String("cardinal.default.topic", "", "Default topic for Cardinal broker")
	blockTopic = Flags.String("cardinal.block.topic", "", "Topic for Cardinal block data")
	logTopic = Flags.String("cardinal.logs.topic", "", "Topic for Cardinal log data")
	txTopic = Flags.String("cardinal.tx.topic", "", "Topic for Cardinal transaction data")
	receiptTopic = Flags.String("cardinal.receipt.topic", "", "Topic for Cardinal receipt data")
	codeTopic = Flags.String("cardinal.code.topic", "", "Topic for Cardinal contract code")
	stateTopic = Flags.String("cardinal.state.topic", "", "Topic for Cardinal state data")
	startBlockOverride = Flags.Uint64("cardinal.start.block", 0, "The first block to emit")
	reorgThreshold = Flags.Int("cardinal.reorg.threshold", 128, "The number of blocks for clients to support quick reorgs")
	statsdaddr = Flags.String("cardinal.statsd.addr", "", "UDP address for a statsd endpoint")
	cloudwatchns = Flags.String("cardinal.cloudwatch.namespace", "", "CloudWatch Namespace for cardinal metrics")
)

func Initialize(ctx *cli.Context, loader core.PluginLoader, logger core.Logger) {
	ready.Add(1)
	log = logger
	log.Info("Cardinal EVM plugin initializing")
	pendingReorgs = make(map[core.Hash]func())
}

func strPtr(x string) *string {
	return &x
}

func InitializeNode(stack core.Node, b restricted.Backend) {
	backend = b
	defer ready.Done()
	config = b.ChainConfig()
	chainid = config.ChainID.Int64()
	if *defaultTopic == "" { *defaultTopic = fmt.Sprintf("cardinal-%v", chainid) }
	if *blockTopic == "" { *blockTopic = fmt.Sprintf("%v-block", *defaultTopic) }
	if *logTopic == "" { *logTopic = fmt.Sprintf("%v-logs", *defaultTopic) }
	if *txTopic == "" { *txTopic = fmt.Sprintf("%v-tx", *defaultTopic) }
	if *receiptTopic == "" { *receiptTopic = fmt.Sprintf("%v-receipt", *defaultTopic) }
	if *codeTopic == "" { *codeTopic = fmt.Sprintf("%v-code", *defaultTopic) }
	if *stateTopic == "" { *stateTopic = fmt.Sprintf("%v-state", *defaultTopic) }
	var err error
	if strings.HasPrefix(*brokerURL, "kafka://") {
		producer, err = transports.NewKafkaProducer(
			strings.TrimPrefix(*brokerURL, "kafka://"),
			*defaultTopic,
			map[string]string{
				fmt.Sprintf("c/%x/a/", chainid): *stateTopic,
				fmt.Sprintf("c/%x/s", chainid): *stateTopic,
				fmt.Sprintf("c/%x/c/", chainid): *codeTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/h", chainid): *blockTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/d", chainid): *blockTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/u/", chainid): *blockTopic,
				fmt.Sprintf("c/%x/n/", chainid): *blockTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/t/", chainid): *txTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/r/", chainid): *receiptTopic,
				fmt.Sprintf("c/%x/b/[0-9a-z]+/l/", chainid): *logTopic,
			},
		)
		if err != nil { panic(err.Error()) }
	}
	if *brokerURL != "" {
		go func() {
			t := time.NewTicker(time.Second * 30)
			defer t.Stop()
			for range t.C {
				gethPeersGauge.Update(int64(stack.Server().PeerCount()))
			}
		}()
		if *statsdaddr != "" {
			udpAddr, err := net.ResolveUDPAddr("udp", *statsdaddr)
			if err != nil {
				log.Error("Invalid Address. Statsd will not be configured.", "error", err.Error())
			}
			go statsd.StatsD(
				metrics.MajorRegistry,
				20 * time.Second,
				"cardinal.geth.master",
				udpAddr,
			)
		}
		if *cloudwatchns != "" {
			go cloudmetrics.Publish(metrics.MajorRegistry,
				*cloudwatchns,
				cloudmetrics.Dimensions("chainid", fmt.Sprintf("%v", chainid)),
				cloudmetrics.Interval(30 * time.Second),
			)
		}
		if *startBlockOverride > 0 {
			startBlock = *startBlockOverride
		} else {
			v, err := producer.LatestBlockFromFeed()
			if err != nil {
				log.Error("Error getting start block", "err", err)
			} else {
				if v > 128 {
					startBlock = uint64(v) - 128
					log.Info("Setting start block from producer", "block", startBlock)
				}
			}
		}
		if *txPoolTopic != "" {
			go func() {
				// TODO: we should probably do something within Cardinal streams to
				// generalize this so it's not Kafka specific and can work with other
				// transports.
				ch := make(chan core.NewTxsEvent, 1000)
				sub := b.SubscribeNewTxsEvent(ch)
				brokers, config := transports.ParseKafkaURL(strings.TrimPrefix(*brokerURL, "kafka://"))
				configEntries := make(map[string]*string)
				configEntries["retention.ms"] = strPtr("3600000")
				if err := transports.CreateTopicIfDoesNotExist(strings.TrimPrefix(*brokerURL, "kafka://"), *txPoolTopic, 0, configEntries); err != nil {
					panic(fmt.Sprintf("Could not create topic %v on broker %v: %v", *txPoolTopic, *brokerURL, err.Error()))
				}
				// This is about twice the size of the largest possible transaction if
				// all gas in a block were zero bytes in a transaction's data. It should
				// be very rare for messages to even approach this size.
				config.Producer.MaxMessageBytes = 10000024
				producer, err := sarama.NewAsyncProducer(brokers, config)
				if err != nil {
					panic(fmt.Sprintf("Could not setup producer: %v", err.Error()))
				}
				for {
					select {
					case txEvent := <-ch:
						for _, txBytes := range txEvent.Txs {
							// Switch from MarshalBinary to RLP encoding to match EtherCattle's legacy format for txpool transactions
							tx := &types.Transaction{}
							if err := tx.UnmarshalBinary(txBytes); err != nil {
								log.Error("Error unmarshalling")
								continue
							}
							txdata, err := rlp.EncodeToBytes(tx)
							if err != nil {
								select {
								case producer.Input() <- &sarama.ProducerMessage{Topic: *txPoolTopic, Value: sarama.ByteEncoder(txdata)}:
								case err := <-producer.Errors():
									log.Error("Error emitting: %v", "err", err.Error())
								}
							} else {
								log.Warn("Error RLP encoding transactions", "err", err)
							}
						}
					case err := <-sub.Err():
						log.Error("Error processing event transactions", "error", err)
						close(ch)
						sub.Unsubscribe()
						return
					}
				}
			}()
		}
	}
	log.Info("Cardinal EVM plugin initialized")

}

type receiptMeta struct {
	ContractAddress core.Address
	CumulativeGasUsed uint64
	GasUsed uint64
	LogsBloom []byte
	Status uint64
	LogCount uint
	LogOffset uint
}

func BUPreReorg(common core.Hash, oldChain []core.Hash, newChain []core.Hash) {
	blockRLP, err := backend.BlockByHash(context.Background(), common)
	if err != nil {
		log.Error("Could not get block for reorg", "hash", common, "err", err)
		return
	}
	var block types.Block
	if err := rlp.DecodeBytes(blockRLP, &block); err != nil {
		log.Error("Could not decode block during reorg", "hash", common, "err", err)
		return
	}
	if len(oldChain) > *reorgThreshold && len(newChain) > 0 {
		pendingReorgs[common], err = producer.Reorg(int64(block.NumberU64()), ctypes.Hash(common))
		if err != nil {
			log.Error("Could not start producer reorg", "block", common, "num", block.NumberU64(), "err", err)
		}
	}
}

func BUPostReorg(common core.Hash, oldChain []core.Hash, newChain []core.Hash) {
	if done, ok := pendingReorgs[common]; ok {
		done()
		delete(pendingReorgs, common)
	}
}

func BlockUpdates(block *types.Block, td *big.Int, receipts types.Receipts, destructs map[core.Hash]struct{}, accounts map[core.Hash][]byte, storage map[core.Hash]map[core.Hash][]byte, code map[core.Hash][]byte) {
	if producer == nil {
		panic("Unknown broker. Please set --cardinal.broker.url")
	}
	ready.Wait()
	hash := block.Hash()
	if block.NumberU64() < startBlock {
		log.Debug("Skipping block production", "current", block.NumberU64(), "start", startBlock)
		return
	}
	headerBytes, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		log.Error("Error parsing header", "block", block.Hash(), "err", err)
		return
	}
	updates := map[string][]byte{
		fmt.Sprintf("c/%x/b/%x/h", chainid, hash.Bytes()): headerBytes,
		fmt.Sprintf("c/%x/b/%x/d", chainid, hash.Bytes()): td.Bytes(),
		fmt.Sprintf("c/%x/n/%x", chainid, block.Number().Int64()): hash[:],
	}
	for i, tx := range block.Transactions() {
		updates[fmt.Sprintf("c/%x/b/%x/t/%x", chainid, hash.Bytes(), i)], err = tx.MarshalBinary()
		if err != nil {
			log.Error("Error marshalling tx", "block", block.Hash(), "tx", i, "err", err)
			return
		}
		rmeta := receiptMeta{
			ContractAddress: core.Address(receipts[i].ContractAddress),
			CumulativeGasUsed: receipts[i].CumulativeGasUsed,
			GasUsed: receipts[i].GasUsed,
			LogsBloom: receipts[i].Bloom.Bytes(),
			Status: receipts[i].Status,
			LogCount: uint(len(receipts[i].Logs)),
		}
		if rmeta.LogCount > 0 {
			rmeta.LogOffset = receipts[i].Logs[0].Index
		}
		updates[fmt.Sprintf("c/%x/b/%x/r/%x", chainid, hash.Bytes(), i)], err = rlp.EncodeToBytes(rmeta)
		if err != nil {
			log.Error("Error marshalling tx receipt", "block", block.Hash(), "tx", i, "err", err)
			return
		}

		for _, logRecord := range receipts[i].Logs {
			updates[fmt.Sprintf("c/%x/b/%x/l/%x/%x", chainid, hash.Bytes(), i, logRecord.Index)], err = rlp.EncodeToBytes(logRecord)
			if err != nil {
				log.Error("Error unmarshalling log record", "block", block.Hash(), "tx", i, "")
			}
		}
	}
	for hashedAddr, acctRLP := range accounts {
		updates[fmt.Sprintf("c/%x/a/%x/d", chainid, hashedAddr.Bytes())] = acctRLP
	}
	for codeHash, codeBytes := range code {
		updates[fmt.Sprintf("c/%x/c/%x", chainid, codeHash.Bytes())] = codeBytes
	}
	deletes := make(map[string]struct{})
	for hashedAddr := range destructs {
		deletes[fmt.Sprintf("c/%x/a/%x", chainid, hashedAddr.Bytes())] = struct{}{}
	}
	batches := map[string]ctypes.Hash{
		fmt.Sprintf("c/%x/s", chainid): ctypes.BigToHash(block.Number()),
	}
	if len(block.Uncles()) > 0 {
		// If uncles == 0, we can figure that out from the hash without having to
		// send an empty list across the wire
		for i, uncle := range block.Uncles() {
			updates[fmt.Sprintf("c/%x/b/%x/u/%x", chainid, hash.Bytes(), i)], err = rlp.EncodeToBytes(uncle)
			if err != nil {
				log.Error("Error marshalling uncles list", "block", block.Hash(), "err", err)
				return
			}
		}
	}
	log.Info("Producing block to kafka", "hash", hash, "number", block.NumberU64())
	gethHeightGauge.Update(block.Number().Int64())
	masterHeightGauge.Update(block.Number().Int64())
	if err := producer.AddBlock(
		block.Number().Int64(),
		ctypes.Hash(hash),
		ctypes.Hash(block.ParentHash()),
		td,
		updates,
		deletes,
		batches,
	); err != nil {
		log.Error("Failed to send block", "block", hash, "err", err)
		panic(err.Error())
		return
	}
	batchUpdates := make(map[string][]byte)
	for addrHash, updates := range storage {
		for k, v := range updates {
			batchUpdates[fmt.Sprintf("c/%x/a/%x/s/%x", chainid, addrHash.Bytes(), k.Bytes())] = v
		}
	}
	if err := producer.SendBatch(ctypes.BigToHash(block.Number()), []string{}, batchUpdates); err != nil {
		log.Error("Failed to send state batch", "block", hash, "err", err)
		return
	}
}
