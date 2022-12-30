package streams

import (
	"github.com/openrelayxyz/cardinal-streams/delivery"
	"github.com/openrelayxyz/cardinal-streams/transports"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/metrics"
	etypes "github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"fmt"
	"regexp"
	"time"
	log "github.com/inconshreveable/log15"
)

var (
	heightGauge = metrics.NewMajorGauge("/evm/height")
)

func BlockTime(pb *delivery.PendingBatch, chainid int64) *time.Time {
	if data, ok := pb.Values[string(schema.BlockHeader(chainid, pb.Hash.Bytes()))]; ok {
		header := etypes.Header{}
		if err := rlp.DecodeBytes(data, &header); err != nil { return nil }
		t := time.Unix(int64(header.Time), 0)
		return &t
	}
	return nil
}

type StreamManager struct{
	consumer transports.Consumer
	storage  storage.Storage
	sub      types.Subscription
	reorgSub types.Subscription
	ready    chan struct{}
	processed uint64
	chainid int64
}

func NewStreamManager(brokerParams []transports.BrokerParams, reorgThreshold, chainid int64, s storage.Storage, whitelist map[uint64]types.Hash, resumptionTime int64) (*StreamManager, error) {
	lastHash, lastNumber, lastWeight, resumption := s.LatestBlock()
	trackedPrefixes := []*regexp.Regexp{
		regexp.MustCompile("c/[0-9a-z]+/a/"),
		regexp.MustCompile("c/[0-9a-z]+/s"),
		regexp.MustCompile("c/[0-9a-z]+/c/"),
		regexp.MustCompile("c/[0-9a-z]+/b/[0-9a-z]+/h"),
		regexp.MustCompile("c/[0-9a-z]+/b/[0-9a-z]+/d"),
		regexp.MustCompile("c/[0-9a-z]+/n/"),
	}
	if resumptionTime < 0 {
		if err := s.View(lastHash, func(tx storage.Transaction) error {
			return tx.ZeroCopyGet(schema.BlockHeader(chainid, lastHash.Bytes()), func(data []byte) error {
				header := etypes.Header{}
				if err := rlp.DecodeBytes(data, &header); err != nil { return err }
				resumptionTime = int64(header.Time) * 1000
				return nil
			})
		}); err != nil {
			log.Warn("Error getting last block timestamp", "err", err)
		}
	}
	var consumer transports.Consumer
	if resumptionTime > 0 {
		r, err := transports.ResumptionForTimestamp(brokerParams, resumptionTime)
		if err != nil {
			log.Warn("Failed to generate resumption from timestamp.", "err", err.Error())
		} else {
			resumption = r
		}
	}
	var err error
	consumer, err = transports.ResolveMuxConsumer(brokerParams, resumption, int64(lastNumber), lastHash, lastWeight, reorgThreshold, trackedPrefixes, whitelist)
	if err != nil { return nil, err }
	return &StreamManager{
		consumer: consumer,
		storage: s,
		ready: make(chan struct{}),
		chainid: chainid,
	}, nil
}

func (m *StreamManager) Start() error {
	if m.sub != nil || m.reorgSub != nil {
		return fmt.Errorf("already started")
	}
	ch := make(chan *delivery.ChainUpdate)
	reorgCh := make(chan map[int64]types.Hash)
	m.sub = m.consumer.Subscribe(ch)
	m.reorgSub = m.consumer.SubscribeReorg(reorgCh)
	go func() {
		<-m.consumer.Ready()
		m.ready <- struct{}{}
	}()
	go func() {
		for {
			log.Debug("Waiting for message")
			select {
			case update := <-ch:
				start := time.Now()
				added := update.Added()
				for _, pb := range added {
					updates := make([]storage.KeyValue, 0, len(pb.Values))
					for k, v := range pb.Values {
						updates = append(updates, storage.KeyValue{Key: []byte(k), Value: v})
					}
					deletes := make([][]byte, 0, len(pb.Deletes))
					for k := range pb.Deletes {
						deletes = append(deletes, []byte(k))
					}
					if err := m.storage.AddBlock(
						pb.Hash,
						pb.ParentHash,
						uint64(pb.Number),
						pb.Weight,
						updates,
						deletes,
						[]byte(pb.Resumption()),
					); err != nil {
						log.Error("Error adding block", "block", pb.Hash, "number", pb.Number, "error", err)
					} else {
						heightGauge.Update(pb.Number)
						m.processed++
					}
					pb.Done()
				}
				latest := added[len(added) - 1]
				params := []interface{}{"blocks", len(added), "elapsed", time.Since(start), "number", latest.Number, "hash", latest.Hash}
				if bt := BlockTime(latest, m.chainid); bt != nil && time.Since(*bt) > time.Minute {
					params = append(params, "age", time.Since(*bt))
				}
				log.Info("Imported new chain segment", params...)
			case reorg := <-reorgCh:
				for k := range reorg {
					m.storage.Rollback(uint64(k))
				}
			}
		}
	}()
	return m.consumer.Start()
}

func (m *StreamManager) Ready() chan struct{}{
	return m.ready
}

func (m *StreamManager) Close() {
	m.sub.Unsubscribe()
	m.reorgSub.Unsubscribe()
	m.consumer.Close()
}

func (m *StreamManager) API() *api {
	return &api{m.consumer}
}

func (m *StreamManager) Processed() uint64 {
	return m.processed
}

type api struct{
	consumer transports.Consumer
}

func (a *api) WhyNotReady(hash types.Hash) string {
	return a.consumer.WhyNotReady(hash)
}
