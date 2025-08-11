package api

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"
	"context"
	"errors"

	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-evm/schema"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-storage"

	ctypes "github.com/openrelayxyz/cardinal-types"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

// defaultTraceTimeout is the amount of time a single transaction can execute
// by default before being forcefully aborted.
const	defaultTraceTimeout = 5 * time.Second

type PrivateDebugAPI struct {
	storage storage.Storage
	evmmgr  *vm.EVMManager
	chainid int64
}

type evmfunc func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM

// Context contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type TracerContext struct {
	BlockHash   ctypes.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int    // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int         // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      ctypes.Hash // Hash of the transaction being traced (zero if dangling call)
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
	// Config specific to given tracer. Note struct logger
	// config are historically embedded in main object.x
	TracerConfig json.RawMessage
}

// TraceCallConfig is the config for traceCall API. It holds one more
// field to override the state for tracing.
type TraceCallConfig struct {
	TraceConfig
	StateOverrides *StateOverride
	TxIndex        *hexutil.Uint
}


// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewDebugAPI(s storage.Storage, evmmgr *vm.EVMManager, chainid int64) *PrivateDebugAPI {
	return &PrivateDebugAPI{storage: s, evmmgr: evmmgr, chainid: chainid}
}

// CreateAccessList creates a EIP-2930 type AccessList for the given transaction.
// Reexec and BlockNrOrHash can be specified to create the accessList on top of a certain state.
func (s *PrivateDebugAPI) TraceStructLog(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash) ([]vm.StructLog, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	var result []vm.StructLog
	err := s.evmmgr.View(bNrOrHash, args.From, ctx, func(header *types.Header, statedb state.StateDB, evmFn func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chaincfg *params.ChainConfig) error {
		var err error
		result, err = TraceStructLog(ctx, statedb, header, chaincfg, evmFn, bNrOrHash, args)
		return err
	})
	if err != nil {
		switch err.(type) {
		case codedError:
		default:
			err = evmError{err}
		}
	}
	return result, err
}

// AccessList creates an access list for the given transaction.
// If the accesslist creation fails an error is returned.
// If the transaction itself fails, an vmErr is returned.
func TraceStructLog(ctx *rpc.CallContext, db state.StateDB, header *types.Header, chaincfg *params.ChainConfig, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, blockNrOrHash vm.BlockNumberOrHash, args TransactionArgs) (slog []vm.StructLog, err error) {
	if err := args.setDefaults(ctx, getEVM, db, header, blockNrOrHash); err != nil {
		return nil, err
	}
	// Create an initial tracer
	tracer := vm.NewStructLogger(&vm.LogConfig{})
	// Get a copy of the statedb primed for calculating the access list
	value := args.Value.ToInt()
	if value == nil { value = new(big.Int) }
	gasPrice := args.GasPrice.ToInt()
	if gasPrice == nil { gasPrice = new(big.Int) }
	msg := NewMessage(args.from(), args.To, uint64(*args.Nonce), value, uint64(*args.Gas), gasPrice, big.NewInt(0), big.NewInt(0), args.data(), *args.AccessList, args.AuthList, false)

	_, err = ApplyMessage(getEVM(db.Copy(), &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, args.from(), msg.GasPrice()), msg, new(GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction: err: %v", err)
	}
	return tracer.StructLogs(), nil
}

// TraceCall lets you trace a given eth_call. It collects the structured logs
// created during the execution of EVM if the given transaction was added on
// top of the provided block and returns them as a JSON object.
// If no transaction index is specified, the trace will be conducted on the state
// after executing the specified block. However, if a transaction index is provided,
// the trace will be conducted on the state after executing the specified transaction
// within the specified block.
func (s *PrivateDebugAPI) TraceCall(ctx *rpc.CallContext, args TransactionArgs, blockNrOrHash *vm.BlockNumberOrHash, config *TraceCallConfig) (interface{}, error) {
	bNrOrHash := vm.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	var (
		statedb state.StateDB
		header *types.Header
		chaincfg *params.ChainConfig
		evmFn evmfunc
	)

	if config != nil  && config.TxIndex != nil {
		var err error
		statedb, header, evmFn, chaincfg, err = s.stateAtTransaction(ctx, bNrOrHash, int(*config.TxIndex))
		if err != nil {
			return nil, err
		}
	} else {
		err := s.evmmgr.View(bNrOrHash, args.from, ctx, func(h *types.Header, sdb state.StateDB, getEVM func (state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, cfg *params.ChainConfig) error{
			statedb = sdb
			header = h
			chaincfg = cfg
			evmFn = getEVM
			return nil
		})
		if err != nil {
			switch err.(type) {
			case codedError:
			default:
				err = evmError{err}
			}
			return nil, err
		}
	}

	if config != nil && config.StateOverrides != nil {
		if err := config.StateOverrides.Apply(statedb); err != nil {
			return nil, err
		}
	}

	gasCap := new(GasPool).AddGas(header.GasLimit)
	if err := args.CallDefaults(gasCap.Gas(), header.BaseFee, chaincfg.ChainID); err != nil {
		return nil, err
	}
	msg, err := args.ToMessage(gasCap.Gas(), header.BaseFee) 
	if err != nil {
		return nil, err
	}
	tx := args.ToTransaction(types.LegacyTxType)
	if msg.GasPrice().Sign() == 0 {
		header.BaseFee = new(big.Int)
	}
	var traceConfig *TraceConfig
	if config != nil {
		traceConfig = &config.TraceConfig
	}
	return s.traceTx(ctx, tx, msg, new(TracerContext), header, statedb, evmFn, traceConfig)
}

// traceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (s *PrivateDebugAPI) traceTx(ctx *rpc.CallContext, tx *types.Transaction, message Msg, txctx *TracerContext, header *types.Header, statedb state.StateDB, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, config *TraceConfig) (interface{}, error){
	var ( 
		timeout = defaultTraceTimeout
		err error
	)

	if config == nil {
		config = &TraceConfig{}
	}
	if config.Timeout != nil {
		if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
			return nil, err
		}
	}
	tracer := vm.NewStructLogger(&vm.LogConfig{})
	deadlineCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
	defer cancel()
	evm := getEVM(statedb, &vm.Config{Tracer: tracer, Debug: true, NoBaseFee: true}, message.From(), message.GasPrice())

	go func() {
		<-deadlineCtx.Done()
		if deadlineCtx.Err() == context.DeadlineExceeded {
			evm.Cancel()
		}
	}()

	statedb.SetTxContext(txctx.TxHash, txctx.TxIndex)
	result, err := ApplyMessage(evm, message, new(GasPool).AddGas(header.GasLimit))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	if evm.Cancelled() {
		return nil, fmt.Errorf("execution timeout")
	}
	failed := result.Failed()
	returnData := result.ReturnData
	if failed && !errors.Is(result.Err, vm.ErrExecutionReverted) {
		returnData = []byte{}
	}
	
	return struct {
		Gas         uint64           `json:"gas"`
		Failed      bool            `json:"failed"`
		ReturnValue hexutil.Bytes   `json:"returnValue"`
		StructLogs  []vm.StructLog  `json:"structLogs"`
	}{
		Gas:         result.UsedGas, 
		Failed:      failed,
		ReturnValue: returnData,
		StructLogs:  tracer.StructLogs(),
	}, nil

}

func (s *PrivateDebugAPI) stateAtTransaction(ctx *rpc.CallContext, blockNrOrHash vm.BlockNumberOrHash, txIndex int) (state.StateDB, *types.Header, func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, *params.ChainConfig, error){
	var resultState state.StateDB
	var resultHeader *types.Header
	var resultChainCfg *params.ChainConfig
	var resultEvmFn evmfunc
	err := s.evmmgr.View(blockNrOrHash, common.Address{}, ctx, func(header *types.Header, statedb state.StateDB, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, chainCfg *params.ChainConfig) error {
		blockHash := header.Hash()
		if hash, ok := blockNrOrHash.Hash(); ok {
			blockHash = hash
		}
		var transactions []*types.Transaction
		err := s.storage.View(blockHash, func(tx storage.Transaction) error {
			for i := 0; ; i++ {
				txkey := schema.Transaction(s.chainid, blockHash.Bytes(), int64(i))
				txdata, err := tx.Get(txkey)
				if err != nil{ break }

				var transaction types.Transaction
				if err := rlp.DecodeBytes(txdata, &transaction); err != nil {
					return fmt.Errorf("failed to decode transaction %v: %v", i, err)
				}
				transactions = append(transactions, &transaction)
			}
			return nil
		})
		if err!=nil{
			return err
		}
		if txIndex >= len(transactions) {
			return fmt.Errorf("transaction index %v exceeds block transaction count %v", txIndex, len(transactions))
		}

		//copy of the state to modify
		wState := statedb.Copy()
		for i:=0; i < txIndex; i++ {
			tx := transactions[i]
			wState.SetTxContext(tx.Hash(), i)
			signer := types.MakeSigner(chainCfg, header.Number, header.Time)
			from, err := signer.Sender(tx)
			if err != nil {
				return fmt.Errorf("failed to recover sender: %v", err)
			}
			
			msg := NewMessage(from, tx.To(), tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), tx.GasFeeCap(), tx.GasTipCap(), tx.Data(), tx.AccessList(), tx.AuthList(), false)
			_, err = ApplyMessage(getEVM(wState, &vm.Config{NoBaseFee: false}, msg.From(), msg.GasPrice()), msg, new(GasPool).AddGas(tx.Gas()))
			if err!= nil {
				return fmt.Errorf("failed to apply transaction: err: %v", err)
			}

		}
		resultState = wState
		resultHeader = header
		resultChainCfg = chainCfg
		resultEvmFn = getEVM

		return nil
	})
	return resultState,resultHeader, resultEvmFn ,resultChainCfg, err
}