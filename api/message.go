package api

import (
	"math/big"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/types"
)

type Msg struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	gasFeeCap  *big.Int
	gasTipCap  *big.Int
	data       []byte
	accessList types.AccessList
	checkNonce bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice, gasFeeCap, gasTipCap *big.Int, data []byte, accessList types.AccessList, checkNonce bool) Msg {
	return Msg{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		gasFeeCap:  gasFeeCap,
		gasTipCap:  gasTipCap,
		data:       data,
		accessList: accessList,
		checkNonce: checkNonce,
	}
}

func (m Msg) From() common.Address   { return m.from }
func (m Msg) To() *common.Address    { return m.to }
func (m Msg) GasPrice() *big.Int     { return m.gasPrice }
func (m Msg) GasFeeCap() *big.Int    { return m.gasFeeCap }
func (m Msg) GasTipCap() *big.Int    { return m.gasTipCap }
func (m Msg) Value() *big.Int        { return m.amount }
func (m Msg) Gas() uint64            { return m.gasLimit }
func (m Msg) Nonce() uint64          { return m.nonce }
func (m Msg) Data() []byte           { return m.data }
func (m Msg) AccessList() types.AccessList { return m.accessList }
func (m Msg) CheckNonce() bool       { return m.checkNonce }
