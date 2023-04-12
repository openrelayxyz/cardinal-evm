package main

import (
	"bytes"
	"net/http"
	"context"
	"fmt"
	"encoding/json"
	"github.com/openrelayxyz/cardinal-streams/transports"
	// "github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/hashicorp/golang-lru"
	"github.com/Shopify/sarama"
	"github.com/inconshreveable/log15"
	"os"
	"time"
	"log"
	"io/ioutil"
)

// replica starts replica node
func main() {
	sarama.Logger = log.New(os.Stderr, "[sarama]", 0)
	rpcEndpoint := os.Args[1]
	brokerURL := os.Args[2]
	topic := os.Args[3]
	consumerGroupID := os.Args[4]
	brokers, config := transports.ParseKafkaURL(brokerURL)
	if err := transports.CreateTopicIfDoesNotExist(brokerURL, topic, 6, nil); err != nil {
		log15.Error("Error creating topic", "err", err)
		return
	}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerGroup, err := sarama.NewConsumerGroup(brokers, consumerGroupID, config)
	if err != nil {
		log15.Error("Error connecting to broker", "url", brokerURL, "err", err)
		return
	}
	defer consumerGroup.Close()
	cache, _ := lru.New(512)
	for {
		handler := relayConsumerGroup{
			url: rpcEndpoint,
			cache: cache,
		}
		if err := consumerGroup.Consume(context.Background(), []string{topic}, handler); err != nil {
			log15.Error("Error consuming", "err", err, "topic", topic)
		}
		time.Sleep(500 * time.Millisecond)
	}
}


type relayConsumerGroup struct{
	url string
	cache *lru.Cache
}

func (relayConsumerGroup) Setup(_ sarama.ConsumerGroupSession) error	 { return nil }
func (relayConsumerGroup) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h relayConsumerGroup) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		tx := new(types.Transaction)
		if err := rlp.DecodeBytes(msg.Value, tx); err != nil {
			log15.Error("Error decoding", "err", err)
			continue
		}
		bin, err := tx.MarshalBinary()
		if err != nil {
			log15.Error("Error MarshalBinary", "err", err)
			continue
		}
		hash := tx.Hash()
		if ok, _ := h.cache.ContainsOrAdd(hash, struct{}{}); !ok {
			log15.Debug("Sending Transaction", "hash", hash, "data", fmt.Sprintf("%#x", bin))
			resp, err := http.Post(h.url, "application/json", bytes.NewBuffer([]byte(fmt.Sprintf(`{"id": 0, "jsonrpc": "2.0", "method": "eth_sendRawTransaction", "params": ["%#x"]}`, bin))))
			if err != nil {
				log15.Error("Error relaying", "tx", hash, "err", err)
				continue
			}
			data, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			var res rpc.Response
			if err := json.Unmarshal(data, &res); err != nil {
				log15.Error("Error unmarshalling JSON response", "err", err, "data", string(data))
				continue
			}
			if res.Error != nil {
				log15.Error("Error relaying transaction", "tx", hash, "err", res.Error)
				continue
			}
			log15.Info("Relaying transaction", "tx", hash, "status", resp.StatusCode)
		} else {
			log15.Info("Skipping repeat", "tx", hash)
		}
		sess.MarkMessage(msg, "")
	}
	return nil
}

type KafkaTransactionConsumer struct {
	producer sarama.SyncProducer
	// TODO;	sarama.SyncProducer
	topic string
}
