package main

import (
	"fmt"
	"runtime"
	"path"
	"strconv"
	"gopkg.in/yaml.v2"
	"github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-streams/v2/transports"
	"github.com/openrelayxyz/cardinal-evm/api"
	etypes "github.com/openrelayxyz/cardinal-evm/types"
	log "github.com/inconshreveable/log15"
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

type statsdOpts struct {
	Address  string `yaml:"address"`
	Port     string `yaml:"port"`
	Prefix   string `yaml:"prefix"`
	Interval int64  `yaml:"interval.sec"`
	Minor    bool   `yaml:"include.minor"`
}

type cloudwatchOpts struct {
	Namespace   string            `yaml:"namespace"`
	Dimensions  map[string]string `yaml:"dimensions"`
	Interval    int64             `yaml:"interval.sec"`
	Percentiles []float64         `yaml:"percentiles"`
	Minor       bool              `yaml:"include.minor"`
}

type gasLimitOpts struct {
	Type string `yaml:"type"`
	Scalar uint64 `yaml:"scalar"`
}

func (glo *gasLimitOpts) RPCGasLimit() api.RPCGasLimit {
	switch glo.Type {
	case "raw":
		return func(*etypes.Header) uint64 { return glo.Scalar }
	case "block":
		return func(h *etypes.Header) uint64 { 
			cap := glo.Scalar * h.GasLimit 
			if cap < 30000000 {
				cap = 30000000
			}
			return cap
		}
	default:
		log.Warn("Unknown RPCGasLimit Method. Defaulting to 2x block gas limit.", "name", glo.Type)
		return func(h *etypes.Header) uint64 { return 2 * h.GasLimit }
	}
}

type Config struct {
	HttpPort int64 `yaml:"http.port"`
	Concurrency int `yaml:"concurrency"`
	Chainid int64 `yaml:"chainid"`
	ReorgThreshold int64 `yaml:"reorg.threshold"`
	RollbackThreshold uint64 `yaml:"rollback.threshold"`
	VacuumTime int64 `yaml:"vacuum.sec"`
	DataDir string `yaml:"datadir"`
	TransactionTopic string `yaml:"topic.transactions"`
	Whitelist map[uint64]string `yaml:"whitelist"`
	LogLevel int `yaml:"log.level"`
	Brokers []broker `yaml:"brokers"`
	HealthChecks rpc.Checks `yaml:"checks"`
	HealthCheckPort int64 `yaml:"hc.port"`
	Statsd *statsdOpts `yaml:"statsd"`
	CloudWatch *cloudwatchOpts `yaml:"cloudwatch"`
	BlockWaitTime int64 `yaml:"block.wait.ms"`
	GasLimitOpts *gasLimitOpts `yaml:"gas.limit"`
	brokers []transports.BrokerParams
	whitelist map[uint64]types.Hash
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
	if cfg.RollbackThreshold == 0 {
		cfg.RollbackThreshold = 90000
	}
	if cfg.VacuumTime == 0 {
		cfg.VacuumTime = 90
	}
	if cfg.DataDir == "" {
		cfg.DataDir = path.Join(home, ".cardinal", "evm", strconv.Itoa(int(cfg.Chainid)))
	}
	cfg.whitelist = make(map[uint64]types.Hash)
	for k, v := range cfg.Whitelist {
		cfg.whitelist[k] = types.HexToHash(v)
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
	if cfg.GasLimitOpts == nil {
		cfg.GasLimitOpts = &gasLimitOpts{}
	}
	if cfg.GasLimitOpts.Type == "" {
		cfg.GasLimitOpts.Type = "block"
	}
	if cfg.GasLimitOpts.Scalar == 0 {
		switch cfg.GasLimitOpts.Type {
		case "block":
			cfg.GasLimitOpts.Scalar = 2
		case "raw": 
			cfg.GasLimitOpts.Scalar = 30000000
		}
	}
	return &cfg, nil
}
