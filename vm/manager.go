package vm

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	// "github.com/openrelayxyz/cardinal-storage/db"
	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/engine"
	"github.com/openrelayxyz/cardinal-evm/eips/eip4844"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"
	ctypes "github.com/openrelayxyz/cardinal-types"
)

var (
	errorType       = reflect.TypeOf((*error)(nil)).Elem()
	headerType      = reflect.TypeOf((*types.Header)(nil))
	hashType        = reflect.TypeOf((*ctypes.Hash)(nil)).Elem()
	uint64Type      = reflect.TypeOf((*uint64)(nil)).Elem()
	statedbType     = reflect.TypeOf((*state.StateDB)(nil)).Elem()
	evmType         = reflect.TypeOf((*EVM)(nil))
	storageTxType   = reflect.TypeOf((*storage.Transaction)(nil)).Elem()
	evmFnType       = reflect.TypeOf((*func(state.StateDB, *Config, common.Address, *big.Int) *EVM)(nil)).Elem()
	chainConfigType = reflect.TypeOf((*params.ChainConfig)(nil))
)

type EVMManager struct {
	sdbm     *state.StatedbManager
	vmcfg    Config
	chaincfg *params.ChainConfig
}

func NewEVMManager(s storage.Storage, chainid int64, vmcfg Config, chaincfg *params.ChainConfig) *EVMManager {
	return &EVMManager{
		sdbm:     &state.StatedbManager{Storage: s, Chainid: chainid},
		vmcfg:    vmcfg,
		chaincfg: chaincfg,
	}
}

func (mgr *EVMManager) View(inputs ...interface{}) error {
	var hash *ctypes.Hash
	var blockNo *rpc.BlockNumber
	var ctx context.Context
	var callback *reflect.Value
	var sender common.Address
	var gasPrice *big.Int
	var vmcfg *Config
	var blobHashes []ctypes.Hash
	var callMeta *rpc.CallMetadata
	for i, input := range inputs {
		switch v := input.(type) {
		case ctypes.Hash:
			hash = &v
		case BlockNumberOrHash:
			if h, ok := v.Hash(); ok {
				hash = &h
			}
			if n, ok := v.Number(); ok {
				blockNo = &n
			}
		case context.Context:
			ctx = v
		case common.Address:
			sender = v
		case *common.Address:
			if v != nil {
				sender = *v
			}
		case []ctypes.Hash:
			blobHashes = v
		case *big.Int:
			gasPrice = v
		case *Config:
			vmcfg = v
		case *rpc.CallMetadata:
			callMeta = v
		case *rpc.CallContext:
			callMeta = v.Metadata()
		default:
			val := reflect.ValueOf(input)
			if val.Kind() == reflect.Func {
				callback = &val
			} else {
				return fmt.Errorf("unknown parameter of type %v in position %v", reflect.TypeOf(input), i)
			}
		}
	}
	if callback == nil {
		return fmt.Errorf("view provided no callback function")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	getHash := func() ctypes.Hash {
		if hash != nil {
			return *hash
		}
		if blockNo != nil && *blockNo >= 0 {
			h, _ := mgr.sdbm.Storage.NumberToHash(uint64(*blockNo))
			hash = &h
		} else {
			h := mgr.sdbm.Storage.LatestHash()
			log.Debug("No hash provided. Using latest", "hash", h)
			hash = &h
		}
		return *hash
	}
	if callMeta != nil {
		callMeta.Hash = getHash()
	}

	sig := callback.Type()
	needsTx, needsState, needsHeader := false, false, false
	txPos, statePos, headerPos, evmPos, evmfnPos, numPos := -1, -1, -1, -1, -1, -1

	argVals := make([]reflect.Value, sig.NumIn())
	for i := 0; i < sig.NumIn(); i++ {
		switch intype := sig.In(i); intype {
		case hashType:
			argVals[i] = reflect.ValueOf(getHash())
		case chainConfigType:
			argVals[i] = reflect.ValueOf(mgr.chaincfg)
		case storageTxType:
			needsTx = true
			txPos = i
		case uint64Type:
			numPos = i
			needsTx = true
		case headerType:
			needsTx = true
			needsHeader = true
			headerPos = i
		case statedbType:
			needsTx = true
			needsState = true
			statePos = i
		case evmType:
			needsTx = true
			needsHeader = true
			needsState = true
			evmPos = i
		case evmFnType:
			needsTx = true
			needsHeader = true
			needsState = true
			evmfnPos = i
		default:
			return fmt.Errorf("unknown input in callback argument %v", i)
		}
	}
	var out []reflect.Value
	if !needsTx {
		out = callback.Call(argVals)
	} else {
		if err := mgr.sdbm.Storage.View(getHash(), func(tx storage.Transaction) error {
			if txPos >= 0 {
				argVals[txPos] = reflect.ValueOf(tx)
			}
			if numPos >= 0 {
				argVals[numPos] = reflect.ValueOf(tx.HashToNumber(getHash()))
			}
			var header *types.Header
			var statedb state.StateDB
			if needsHeader {
				header = &types.Header{}
				if err := tx.ZeroCopyGet(schema.BlockHeader(mgr.sdbm.Chainid, getHash().Bytes()), func(data []byte) error {
					return rlp.DecodeBytes(data, &header)
				}); err != nil {
					return fmt.Errorf("error getting block header: %v (%v)", err.Error(), string(schema.BlockHeader(mgr.sdbm.Chainid, getHash().Bytes())))
				}
				if headerPos >= 0 {
					argVals[headerPos] = reflect.ValueOf(header)
				}
			}
			if needsState {
				statedb = state.NewStateDB(tx, mgr.sdbm.Chainid)
				if statePos >= 0 {
					argVals[statePos] = reflect.ValueOf(statedb)
				}
			}
			if evmPos >= 0 {
				blockCtx := BlockContext{
					GetHash:     tx.NumberToHash,
					Coinbase:    engine.Author(header, mgr.chaincfg.Engine),
					GasLimit:    header.GasLimit,
					BlockNumber: header.Number,
					Time:        new(big.Int).SetUint64(header.Time),
					Difficulty:  header.Difficulty,
					BaseFee:     header.BaseFee,
					BlobBaseFee: eip4844.CalcBlobFee(header.ExcessBlobGas),
				}
				if gasPrice == nil {
					gasPrice = header.BaseFee
				}
				if vmcfg == nil {
					vmcfg = &mgr.vmcfg
				}
				argVals[evmPos] = reflect.ValueOf(NewEVM(blockCtx, TxContext{sender, gasPrice, blobHashes}, statedb, mgr.chaincfg, *vmcfg))
			}
			if evmfnPos >= 0 {
				// TODO: evmfunc needs to take a gas price variable.
				argVals[evmfnPos] = reflect.ValueOf(func(sdb state.StateDB, cvmcfg *Config, sender common.Address, gp *big.Int) *EVM {
					blockCtx := BlockContext{
						GetHash:     tx.NumberToHash,
						Coinbase:    engine.Author(header, mgr.chaincfg.Engine),
						GasLimit:    header.GasLimit,
						BlockNumber: header.Number,
						Time:        new(big.Int).SetUint64(header.Time),
						Difficulty:  header.Difficulty,
						BaseFee:     header.BaseFee,
						BlobBaseFee: eip4844.CalcBlobFee(header.ExcessBlobGas),
					}
					if header.Difficulty.Cmp(new(big.Int)) == 0 {
						blockCtx.Random = &header.MixDigest
					}
					if gasPrice == nil {
						if gp != nil {
							gasPrice = gp
						} else {
							gasPrice = header.BaseFee
						}
					}
					if vmcfg == nil {
						vmcfg = &mgr.vmcfg
					}
					if cvmcfg == nil {
						cvmcfg = vmcfg
					}
					return NewEVM(blockCtx, TxContext{sender, gasPrice, blobHashes}, sdb, mgr.chaincfg, *cvmcfg)
				})
			}
			out = callback.Call(argVals)
			return nil
		}); err != nil {
			return err
		}
	}
	if len(out) == 0 {
		return nil
	}
	outval := out[0].Interface()
	if outval == nil {
		return nil
	}
	switch v := outval.(type) {
	case error:
		return v
	default:
		log.Warn("Unexpected return type in view function", "type", reflect.TypeOf(out[0].Interface()))
		return nil
	}
}
