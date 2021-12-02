package streams

import (
	"github.com/openrelayxyz/cardinal-streams/delivery"
	"github.com/openrelayxyz/cardinal-streams/transports"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types"
	"fmt"
	"regexp"
	log "github.com/inconshreveable/log15"
)

type StreamManager struct{
	consumer transports.Consumer
	storage  storage.Storage
	sub      types.Subscription
	reorgSub types.Subscription
	ready    chan struct{}
}

func NewStreamManager(brokerParams []transports.BrokerParams, reorgThreshold, chainid int64, s storage.Storage, whitelist map[uint64]types.Hash) (*StreamManager, error) {
	lastHash, lastNumber, lastWeight, resumption := s.LatestBlock()
	trackedPrefixes := []*regexp.Regexp{
		regexp.MustCompile("c/[0-9a-z]+/a/"),
		regexp.MustCompile("c/[0-9a-z]+/s"),
		regexp.MustCompile("c/[0-9a-z]+/c/"),
		regexp.MustCompile("c/[0-9a-z]+/b/[0-9a-z]+/h"),
		regexp.MustCompile("c/[0-9a-z]+/b/[0-9a-z]+/d"),
		regexp.MustCompile("c/[0-9a-z]+/n/"),
	}
	var consumer transports.Consumer
	var err error
	consumer, err = transports.ResolveMuxConsumer(brokerParams, resumption, int64(lastNumber), lastHash, lastWeight, reorgThreshold, trackedPrefixes, whitelist)
	if err != nil { return nil, err }
	return &StreamManager{
		consumer: consumer,
		storage: s,
		ready: make(chan struct{}),
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
				for _, pb := range update.Added() {
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
					}
					pb.Done()
				}
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

type api struct{
	consumer transports.Consumer
}

func (a *api) WhyNotReady(hash types.Hash) string {
	return a.consumer.WhyNotReady(hash)
}
