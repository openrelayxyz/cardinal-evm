package txemitter

import (
	"github.com/Shopify/sarama"
	// "fmt"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-streams/transports"
	"strings"
	// log "github.com/inconshreveable/log15"
)

type KafkaTransactionProducer struct {
	producer sarama.SyncProducer
	topic    string
}

func strPtr(x string) *string { return &x }

func NewKafkaTransactionProducerFromURLs(brokerURL, topic string) (TransactionProducer, error) {
	configEntries := make(map[string]*string)
	configEntries["retention.ms"] = strPtr("3600000")
	brokers, config := transports.ParseKafkaURL(brokerURL)
	if err := transports.CreateTopicIfDoesNotExist(strings.TrimPrefix(brokerURL, "kafka://"), topic, 0, configEntries); err != nil {
		return nil, err
	}
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = 5
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return NewKafkaTransactionProducer(producer, topic), nil
}

func NewKafkaTransactionProducer(producer sarama.SyncProducer, topic string) TransactionProducer {
	return &KafkaTransactionProducer{producer, topic}
}

func (producer *KafkaTransactionProducer) Close() {
	producer.producer.Close()
}

func (producer *KafkaTransactionProducer) Emit(tx *types.Transaction) error {
	txBytes, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{Topic: producer.topic, Value: sarama.ByteEncoder(txBytes)}
	_, _, err = producer.producer.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}
