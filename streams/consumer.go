package streams

import (
	"github.com/openrelayxyz/cardinal-streams/v2/delivery"
	"github.com/openrelayxyz/cardinal-streams/v2/transports"
	"github.com/openrelayxyz/cardinal-streams/v2/waiter"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/metrics"
	etypes "github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-rpc"
	"fmt"
	"regexp"
	"math/big"
	"time"
	log "github.com/inconshreveable/log15"
)

var (
	heightGauge = metrics.NewMajorGauge("/evm/height")
	processTimer = metrics.NewMajorTimer("/evm/processing")
	blockAgeTimer = metrics.NewMajorTimer("/evm/age")
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
	lastBlockTime time.Time
	processTime time.Duration
	heightCh chan<- *rpc.HeightRecord
}

func NewStreamManager(brokerParams []transports.BrokerParams, reorgThreshold, chainid int64, s storage.Storage, whitelist map[uint64]types.Hash, resumptionTime int64, heightCh chan<- *rpc.HeightRecord, failedReconstructPanic bool, blacklist map[string]map[int32]map[int64]struct{}) (*StreamManager, error) {
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
	consumer, err = transports.ResolveMuxConsumer(brokerParams, resumption, &delivery.ConsumerConfig{
		LastEmittedNum: int64(lastNumber), 
		LastHash: lastHash, 
		LastWeight: lastWeight, 
		ReorgThreshold: reorgThreshold, 
		TrackedPrefixes: trackedPrefixes, 
		Whitelist: whitelist,
		FailedReconstructPanic: failedReconstructPanic,
		Blacklist: blacklist,
	})
	if err != nil { return nil, err }
	return &StreamManager{
		consumer: consumer,
		storage: s,
		ready: make(chan struct{}),
		chainid: chainid,
		heightCh: heightCh,
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
	safeNumKey := fmt.Sprintf("c/%x/n/safe", m.chainid)
	finalizedNumKey := fmt.Sprintf("c/%x/n/finalized", m.chainid)
	waiting := false
	select {
	case <-m.consumer.Ready():
		waiting = true
	case <-time.After(500*time.Millisecond):
	}

	waitCh := make(chan struct{})
	go func() {
		<-m.consumer.Ready()
		waiting = true
		<-waitCh
		m.ready <- struct{}{}
	}()
	go func() {
		for {
			log.Debug("Waiting for message")
			var delay <-chan time.Time
			if waiting {
				delay = time.After(500 * time.Millisecond)
			}
			select {
			case update := <-ch:
				start := time.Now()
				added := update.Added()
				var safeNum, finalizedNum *big.Int
				for _, pb := range added {
					updates := make([]storage.KeyValue, 0, len(pb.Values))
					for k, v := range pb.Values {
						switch k {
						case safeNumKey:
							safeNum = new(big.Int).SetBytes(v)
						case finalizedNumKey:
							finalizedNum = new(big.Int).SetBytes(v)
						default:
							updates = append(updates, storage.KeyValue{Key: []byte(k), Value: v})
						}
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
				}
				latest := added[len(added) - 1]
				processTimer.UpdateSince(start)
				m.processTime = time.Since(start)
				m.lastBlockTime = time.Now()
				params := []interface{}{"blocks", len(added), "elapsed", m.processTime, "number", latest.Number, "hash", latest.Hash}
				if bt := BlockTime(latest, m.chainid); bt != nil {
					if time.Since(*bt) > time.Minute {
						params = append(params, "age", time.Since(*bt))
					}
					blockAgeTimer.UpdateSince(*bt)
				}
				heightRecord := &rpc.HeightRecord{
					Latest: latest.Number,
				}
				if safeNum != nil {
					i := safeNum.Int64()
					heightRecord.Safe = &i
				}
				if finalizedNum != nil {
					i := finalizedNum.Int64()
					heightRecord.Finalized = &i
				}
				update.Done()
				m.heightCh <- heightRecord
				log.Info("Imported new chain segment", params...)
			case reorg := <-reorgCh:
				for k := range reorg {
					m.storage.Rollback(uint64(k))
				}
			case <-delay:
				// If m.consumer.Ready() has produced a result, waiting will be true, and that goroutine will be
				// waiting on waitCh. delay will be nil unless waiting is true, so this case will only ever resolve
				// if waiting is true and neither update nor reorg has anything ready to produce. When it does resolve,
				// we set waiting back to false so delay will be nil going forward (this case will never resolve again)
				// and close waitCh so that we can finally resolve the ready channel.
				waiting = false
				close(waitCh)
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

func (m *StreamManager) Waiter() waiter.Waiter {
	return m.consumer.Waiter()
}

func (m *StreamManager) API() *api {
	return &api{m.consumer}
}

func (m *StreamManager) Processed() uint64 {
	return m.processed
}

func (m *StreamManager) Healthy() rpc.HealthStatus {
	switch {
	case m.processed == 0:
		return rpc.Unavailable
	case time.Since(m.lastBlockTime) > 60 * time.Second:
		return rpc.Warning
	case m.processTime > 2 * time.Second:
		return rpc.Warning
	}
	return rpc.Healthy
}

type api struct{
	consumer transports.Consumer
}

func (a *api) WhyNotReady(hash types.Hash) string {
	return a.consumer.WhyNotReady(hash)
}
