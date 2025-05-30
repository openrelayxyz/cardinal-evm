// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package api

import (
	"errors"
	"math/big"

	log "github.com/inconshreveable/log15"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/common/math"
	"github.com/openrelayxyz/cardinal-evm/state"
	"github.com/openrelayxyz/cardinal-evm/types"
	"github.com/openrelayxyz/cardinal-evm/vm"
	"github.com/openrelayxyz/cardinal-rpc"
	"github.com/openrelayxyz/cardinal-types/hexutil"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`

	AuthList   []types.Authorization `json:"authorizationList,omitempty"`
}

// from retrieves the transaction sender address.
func (arg *TransactionArgs) from() common.Address {
	if arg.From == nil {
		return common.Address{}
	}
	return *arg.From
}

// data retrieves the transaction calldata. Input field is preferred.
func (arg *TransactionArgs) data() []byte {
	if arg.Input != nil {
		return *arg.Input
	}
	if arg.Data != nil {
		return *arg.Data
	}
	return nil
}

// ToMessage converts th transaction arguments to the Message type used by the
// core evm. This method is used in calls and traces that do not require a real
// live transaction.
func (args *TransactionArgs) ToMessage(globalGasCap uint64, baseFee *big.Int) (Msg, error) {
	// Reject invalid combinations of pre- and post-1559 fee styles
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return Msg{}, errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	}
	// Set sender address or use zero address if none specified.
	addr := args.from()

	// Set default gas & gas price if none were set
	gas := globalGasCap
	if gas == 0 {
		gas = uint64(math.MaxUint64 / 2)
	}
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != 0 && globalGasCap < gas {
		log.Warn("Caller gas above allowance, capping", "requested", gas, "cap", globalGasCap)
		gas = globalGasCap
	}
	var (
		gasPrice  *big.Int
		gasFeeCap *big.Int
		gasTipCap *big.Int
	)
	if baseFee == nil {
		// If there's no basefee, then it must be a non-1559 execution
		gasPrice = new(big.Int)
		if args.GasPrice != nil {
			gasPrice = args.GasPrice.ToInt()
		}
		gasFeeCap, gasTipCap = gasPrice, gasPrice
	} else {
		// A basefee is provided, necessitating 1559-type execution
		if args.GasPrice != nil {
			// User specified the legacy gas field, convert to 1559 gas typing
			gasPrice = args.GasPrice.ToInt()
			gasFeeCap, gasTipCap = gasPrice, gasPrice
		} else {
			// User specified 1559 gas feilds (or none), use those
			gasFeeCap = new(big.Int)
			if args.MaxFeePerGas != nil {
				gasFeeCap = args.MaxFeePerGas.ToInt()
			}
			gasTipCap = new(big.Int)
			if args.MaxPriorityFeePerGas != nil {
				gasTipCap = args.MaxPriorityFeePerGas.ToInt()
			}
			// Backfill the legacy gasPrice for EVM execution, unless we're all zeroes
			gasPrice = new(big.Int)
			if gasFeeCap.BitLen() > 0 || gasTipCap.BitLen() > 0 {
				gasPrice = math.BigMin(new(big.Int).Add(gasTipCap, baseFee), gasFeeCap)
			}
		}
	}
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	data := args.data()
	var accessList types.AccessList
	if args.AccessList != nil {
		accessList = *args.AccessList
	}
	msg := NewMessage(addr, args.To, 0, value, gas, gasPrice, gasFeeCap, gasTipCap, data, accessList, args.AuthList, false)
	return msg, nil
}

// setDefaults provides values such that transactions will execute
// successfully. Unlike the go-ethereum verson of this method, this is not
// intended to be sane recommendations for gas prices based on mempool.
func (args *TransactionArgs) setDefaults(ctx *rpc.CallContext, getEVM func(state.StateDB, *vm.Config, common.Address, *big.Int) *vm.EVM, db state.StateDB, header *types.Header, blockNrOrHash vm.BlockNumberOrHash) error {
	if args.From == nil {
		args.From = &(common.Address{})
	}
	// if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
	// 	return errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	// }
	// if args.MaxFeePerGas == nil {
	// 	args.MaxFeePerGas = (*hexutil.Big)(header.BaseFee)
	// }
	if args.GasPrice == nil {
		args.GasPrice = (*hexutil.Big)(header.BaseFee)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Nonce == nil {
		nonce := db.GetNonce(*args.From)
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Gas == nil {
		gas, _, err := DoEstimateGas(ctx, getEVM, *args, &PreviousState{db.ALCalcCopy(), header}, blockNrOrHash, header.GasLimit, true)
		if err != nil {
			return err
		}
		args.Gas = &gas
	}
	return nil
}

//
// // setDefaults fills in default values for unspecified tx fields.
// func (args *TransactionArgs) setDefaults(ctx context.Context, b Backend) error {
// 	// After london, default to 1559 unless gasPrice is set
// 	head := b.CurrentHeader()
// 	if b.ChainConfig().IsLondon(head.Number) && args.GasPrice == nil {
// 		if args.MaxPriorityFeePerGas == nil {
// 			tip, err := b.SuggestGasTipCap(ctx)
// 			if err != nil {
// 				return err
// 			}
// 			args.MaxPriorityFeePerGas = (*hexutil.Big)(tip)
// 		}
// 		if args.MaxFeePerGas == nil {
// 			gasFeeCap := new(big.Int).Add(
// 				(*big.Int)(args.MaxPriorityFeePerGas),
// 				new(big.Int).Mul(head.BaseFee, big.NewInt(2)),
// 			)
// 			args.MaxFeePerGas = (*hexutil.Big)(gasFeeCap)
// 		}
// 		if args.MaxFeePerGas.ToInt().Cmp(args.MaxPriorityFeePerGas.ToInt()) < 0 {
// 			return fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", args.MaxFeePerGas, args.MaxPriorityFeePerGas)
// 		}
// 	} else {
// 		if args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil {
// 			return errors.New("maxFeePerGas or maxPriorityFeePerGas specified but london is not active yet")
// 		}
// 		if args.GasPrice == nil {
// 			price, err := b.SuggestGasTipCap(ctx)
// 			if err != nil {
// 				return err
// 			}
// 			if b.ChainConfig().IsLondon(head.Number) {
// 				// The legacy tx gas price suggestion should not add 2x base fee
// 				// because all fees are consumed, so it would result in a spiral
// 				// upwards.
// 				price.Add(price, head.BaseFee)
// 			}
// 			args.GasPrice = (*hexutil.Big)(price)
// 		}
// 	}
// 	if args.Value == nil {
// 		args.Value = new(hexutil.Big)
// 	}
// 	if args.Nonce == nil {
// 		nonce, err := b.GetPoolNonce(ctx, args.from())
// 		if err != nil {
// 			return err
// 		}
// 		args.Nonce = (*hexutil.Uint64)(&nonce)
// 	}
// 	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
// 		return errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`)
// 	}
// 	if args.To == nil && len(args.data()) == 0 {
// 		return errors.New(`contract creation without any data provided`)
// 	}
// 	// Estimate the gas usage if necessary.
// 	if args.Gas == nil {
// 		// These fields are immutable during the estimation, safe to
// 		// pass the pointer directly.
// 		callArgs := TransactionArgs{
// 			From:                 args.From,
// 			To:                   args.To,
// 			GasPrice:             args.GasPrice,
// 			MaxFeePerGas:         args.MaxFeePerGas,
// 			MaxPriorityFeePerGas: args.MaxPriorityFeePerGas,
// 			Value:                args.Value,
// 			Data:                 args.Data,
// 			AccessList:           args.AccessList,
// 		}
// 		pendingBlockNr := rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
// 		estimated, err := DoEstimateGas(ctx, b, callArgs, pendingBlockNr, b.RPCGasCap())
// 		if err != nil {
// 			return err
// 		}
// 		args.Gas = &estimated
// 		log.Trace("Estimate gas usage automatically", "gas", args.Gas)
// 	}
// 	if args.ChainID == nil {
// 		id := (*hexutil.Big)(b.ChainConfig().ChainID)
// 		args.ChainID = id
// 	}
// 	return nil
// }
