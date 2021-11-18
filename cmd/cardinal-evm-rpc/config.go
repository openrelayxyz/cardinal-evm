package main

import (
	"fmt"
	"runtime"
	"path"
	"strconv"
	"gopkg.in/yaml.v2"
	"github.com/openrelayxyz/cardinal-types"
	"io/ioutil"
	"os"
)

type checks struct {
	Frequency int64 `yaml:"frequency"`
	Payload string  `yaml:"payload"`
	Method string   `yaml:"method"`
	MaxHeavyResponseTime int64 `yaml:"max_heavy_response_time_ms"`
	MaxNormalResponseTime int64 `yaml:"max_normal_response_time_ms"`
}

type broker struct {
	URL string `yaml:"url"`
	DefaultTopic string `yaml:"default.topic"`
	BlockTopic string `yaml:"block.topic"`
	CodeTopic string `yaml:"code.topic"`
	StateTopic string `yaml:"state.topic"`
}

type Config struct {
	HttpPort int64 `yaml:"http.port"`
	Concurrency int `yaml:"concurrency"`
	Chainid int64 `yaml:"chainid"`
	ReorgThreshold int64 `yaml:"reorg.threshold"`
	DataDir string `yaml:"datadir"`
	Rollback int64 `yaml:"rollback"`
	TransactionTopic string `yaml:"topic.transactions"`
	Whitelist map[uint64]types.Hash `yaml:"whitelist"`
	LogLevel int `yaml:"log.level"`
	Brokers []broker `yaml:"brokers"`
	HealthChecks []checks `yaml:"checks"`
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
	if cfg.Rollback == 0 {
		cfg.Rollback = 128
	}
	if cfg.Whitelist == nil {
		cfg.Whitelist = make(map[uint64]types.Hash)
	}
	if cfg.LogLevel == 0 {
		cfg.LogLevel = 2
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("Config must specify at least one broker")
	}
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
	}
	return &cfg, nil
}
