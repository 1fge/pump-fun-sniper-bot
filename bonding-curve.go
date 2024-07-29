package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// BondingCurveData holds the relevant information decoded from the on-chain data.
type BondingCurveData struct {
	RealTokenReserves    *big.Int
	VirtualTokenReserves *big.Int
	VirtualSolReserves   *big.Int
}

func (b *BondingCurveData) String() string {
	return fmt.Sprintf("RealTokenReserves=%s, VirtualTokenReserves=%s, VirtualSolReserves=%s", b.RealTokenReserves, b.VirtualTokenReserves, b.VirtualSolReserves)
}

// fetchBondingCurve fetches the bonding curve data from the blockchain and decodes it.
func (b *Bot) fetchBondingCurve(bondingCurvePubKey solana.PublicKey) (*BondingCurveData, error) {
	accountInfo, err := b.rpcClient.GetAccountInfoWithOpts(context.TODO(), bondingCurvePubKey, &rpc.GetAccountInfoOpts{Encoding: solana.EncodingBase64, Commitment: rpc.CommitmentProcessed})
	if err != nil || accountInfo.Value == nil {
		return nil, fmt.Errorf("FBCD: failed to get account info: %w", err)
	}

	data := accountInfo.Value.Data.GetBinary()
	if len(data) < 24 {
		return nil, fmt.Errorf("FBCD: insufficient data length")
	}

	// Decode the bonding curve data assuming it follows little-endian format
	realTokenReserves := big.NewInt(0).SetUint64(binary.LittleEndian.Uint64(data[0:8]))
	virtualTokenReserves := big.NewInt(0).SetUint64(binary.LittleEndian.Uint64(data[8:16]))
	virtualSolReserves := big.NewInt(0).SetUint64(binary.LittleEndian.Uint64(data[16:24]))

	return &BondingCurveData{
		RealTokenReserves:    realTokenReserves,
		VirtualTokenReserves: virtualTokenReserves,
		VirtualSolReserves:   virtualSolReserves,
	}, nil
}

// calculateBuyQuote calculates how many tokens can be purchased given a specific amount of SOL, bonding curve data, and percentage.
func calculateBuyQuote(solAmount uint64, bondingCurve *BondingCurveData, percentage float64) *big.Int {
	// Convert solAmount to *big.Int
	solAmountBig := big.NewInt(int64(solAmount))

	// Clone bonding curve data to avoid mutations
	virtualSolReserves := new(big.Int).Set(bondingCurve.VirtualSolReserves)
	virtualTokenReserves := new(big.Int).Set(bondingCurve.VirtualTokenReserves)

	// Compute the new virtual reserves
	newVirtualSolReserves := new(big.Int).Add(virtualSolReserves, solAmountBig)
	invariant := new(big.Int).Mul(virtualSolReserves, virtualTokenReserves)
	newVirtualTokenReserves := new(big.Int).Div(invariant, newVirtualSolReserves)

	// Calculate the tokens to buy
	tokensToBuy := new(big.Int).Sub(virtualTokenReserves, newVirtualTokenReserves)

	// Apply the percentage reduction (e.g., 95% or 0.95)
	// Convert the percentage to a multiplier (0.95) and apply to tokensToBuy
	percentageMultiplier := big.NewFloat(percentage)
	tokensToBuyFloat := new(big.Float).SetInt(tokensToBuy)
	finalTokens := new(big.Float).Mul(tokensToBuyFloat, percentageMultiplier)

	// Convert the result back to *big.Int
	finalTokensBig, _ := finalTokens.Int(nil)

	return finalTokensBig
}
