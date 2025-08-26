package eip4844

import (
	"math/big"

	"github.com/openrelayxyz/cardinal-evm/params"
	"github.com/openrelayxyz/cardinal-evm/types"
)

var (
	minBlobGasPrice            = big.NewInt(params.BlobTxMinBlobGasprice)
	blobGaspriceUpdateFraction = big.NewInt(params.BlobTxBlobGaspriceUpdateFraction)
)

// BlobConfig contains the parameters for blob-related formulas.
// These can be adjusted in a fork.
type BlobConfig struct {
	Target         int
	Max            int
	UpdateFraction uint64
}

// blobBaseFee computes the blob fee.
func (bc *BlobConfig) blobBaseFee(excessBlobGas uint64) *big.Int {
	return fakeExponential(minBlobGasPrice, new(big.Int).SetUint64(excessBlobGas), new(big.Int).SetUint64(bc.UpdateFraction))
}


// blobPrice returns the price of one blob in Wei.
func (bc *BlobConfig) blobPrice(excessBlobGas uint64) *big.Int {
	f := bc.blobBaseFee(excessBlobGas)
	return new(big.Int).Mul(f, big.NewInt(params.BlobTxBlobGasPerBlob))
}

func latestBlobConfig(cfg *params.ChainConfig, time uint64) *BlobConfig {
	if cfg.BlobSchedule == nil {
		return nil
	}
	
	var latestConfig *params.BlobConfig
	
	for _, config := range cfg.BlobSchedule {
		if config.ActivationTime <= time {
			latestConfig = config
		}
	}
	
	if latestConfig == nil {
		return nil
	}

	return &BlobConfig{
		Target:         latestConfig.Target,
		Max:            latestConfig.Max,
		UpdateFraction: latestConfig.UpdateFraction,
	}
}

// CalcExcessBlobGas calculates the excess blob gas after applying the set of
// blobs on top of the excess blob gas.
func CalcExcessBlobGas(config *params.ChainConfig, parent *types.Header, headTimestamp uint64) uint64 {
	isOsaka := config.IsOsaka(new(big.Int).SetUint64(headTimestamp), parent.Number)
	bcfg := latestBlobConfig(config, headTimestamp)
	return calcExcessBlobGas(isOsaka, bcfg, parent)
}

func calcExcessBlobGas(isOsaka bool, bcfg *BlobConfig, parent *types.Header) uint64 {
	var parentExcessBlobGas, parentBlobGasUsed uint64
	if parent.ExcessBlobGas != nil {
		parentExcessBlobGas = *parent.ExcessBlobGas
		parentBlobGasUsed = *parent.BlobGasUsed
	}

	var (
		excessBlobGas = parentExcessBlobGas + parentBlobGasUsed
		targetGas     = uint64(bcfg.Target) * params.BlobTxBlobGasPerBlob
	)
	if excessBlobGas < targetGas {
		return 0
	}

	// EIP-7918 (post-Osaka) introduces a different formula for computing excess,
	// in cases where the price is lower than a 'reserve price'.
	if isOsaka {
		var (
			baseCost     = big.NewInt(params.BlobBaseCost)
			reservePrice = baseCost.Mul(baseCost, parent.BaseFee)
			blobPrice    = bcfg.blobPrice(parentExcessBlobGas)
		)
		if reservePrice.Cmp(blobPrice) > 0 {
			scaledExcess := parentBlobGasUsed * uint64(bcfg.Max-bcfg.Target) / uint64(bcfg.Max)
			return parentExcessBlobGas + scaledExcess
		}
	}

	// Original EIP-4844 formula.
	return excessBlobGas - targetGas
}

// CalcBlobFee calculates the blobfee from the header's excess blob gas field.
func CalcBlobFee(excessBlobGas *uint64) *big.Int {
	if excessBlobGas == nil {
		return nil
	}
	return fakeExponential(minBlobGasPrice, new(big.Int).SetUint64(*excessBlobGas), blobGaspriceUpdateFraction)
}

// fakeExponential approximates factor * e ** (numerator / denominator) using
// Taylor expansion.
func fakeExponential(factor, numerator, denominator *big.Int) *big.Int {
	var (
		output = new(big.Int)
		accum  = new(big.Int).Mul(factor, denominator)
	)
	for i := 1; accum.Sign() > 0; i++ {
		output.Add(output, accum)

		accum.Mul(accum, numerator)
		accum.Div(accum, denominator)
		accum.Div(accum, big.NewInt(int64(i)))
	}
	return output.Div(output, denominator)
}