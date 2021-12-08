package main

import (
	"fmt"
	"runtime"
	"path"
	"strconv"
	"gopkg.in/yaml.v2"
	"github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-streams/transports"
	"io/ioutil"
	"os"
)

type broker struct {
	URL string `yaml:"url"`
	DefaultTopic string `yaml:"default.topic"`
	BlockTopic string `yaml:"block.topic"`
	CodeTopic string `yaml:"code.topic"`
	StateTopic string `yaml:"state.topic"`
	Rollback int64 `yaml:"rollback"`
}

type Config struct {
	HttpPort int64 `yaml:"http.port"`
	Concurrency int `yaml:"concurrency"`
	Chainid int64 `yaml:"chainid"`
	ReorgThreshold int64 `yaml:"reorg.threshold"`
	DataDir string `yaml:"datadir"`
	TransactionTopic string `yaml:"topic.transactions"`
	Whitelist map[uint64]types.Hash `yaml:"whitelist"`
	LogLevel int `yaml:"log.level"`
	Brokers []broker `yaml:"brokers"`
	HealthChecks rpc.Checks `yaml:"checks"`
	HealthCheckPort int64 `yaml:"hc.port"`
	brokers []transports.BrokerParams
}

func LoadConfig(fname string) (*Config, error) {
	home, _ := os.UserHomeDir()
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Concurrency == 0 {
		cfg.Concurrency = runtime.NumCPU()*4
	}
	if cfg.Chainid == 0 {
		cfg.Chainid = 1
	}
	if cfg.ReorgThreshold == 0 {
		cfg.ReorgThreshold = 128
	}
	if cfg.DataDir == "" {
		cfg.DataDir = path.Join(home, ".cardinal", "evm", strconv.Itoa(int(cfg.Chainid)))
	}
	if cfg.Whitelist == nil {
		cfg.Whitelist = make(map[uint64]types.Hash)
	}
	if cfg.LogLevel == 0 {
		cfg.LogLevel = 2
	}
	if cfg.HealthCheckPort == 0 {
		cfg.HealthCheckPort = 9999
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("Config must specify at least one broker")
	}
	cfg.brokers = make([]transports.BrokerParams, len(cfg.Brokers))
	for i := range cfg.Brokers {
		if cfg.Brokers[i].DefaultTopic == "" {
			cfg.Brokers[i].DefaultTopic = fmt.Sprintf("cardinal-%v", cfg.Chainid)
		}
		if cfg.Brokers[i].BlockTopic == "" {
			cfg.Brokers[i].BlockTopic = fmt.Sprintf("%v-block", cfg.Brokers[i].DefaultTopic)
		}
		if cfg.Brokers[i].CodeTopic == "" {
			cfg.Brokers[i].CodeTopic = fmt.Sprintf("%v-code", cfg.Brokers[i].DefaultTopic)
		}
		if cfg.Brokers[i].StateTopic == "" {
			cfg.Brokers[i].StateTopic = fmt.Sprintf("%v-state", cfg.Brokers[i].DefaultTopic)
		}
		if cfg.Brokers[i].Rollback == 0 {
			cfg.Brokers[i].Rollback = 5000
		}
		cfg.brokers[i] = transports.BrokerParams{
			URL: cfg.Brokers[i].URL,
			DefaultTopic: cfg.Brokers[i].DefaultTopic,
			Topics: []string{
				cfg.Brokers[i].DefaultTopic,
				cfg.Brokers[i].BlockTopic,
				cfg.Brokers[i].CodeTopic,
				cfg.Brokers[i].StateTopic,
			},
			Rollback: cfg.Brokers[i].Rollback,
		}
	}
	return &cfg, nil
}
