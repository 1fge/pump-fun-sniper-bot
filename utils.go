package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

// increase lookup time for funders with some common exchange addresses
var exchangeAddresses = map[string]interface{}{
	"AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2": nil,
	"42brAgAVNzMBP7aaktPvAmBSPEkehnFQejiZc53EpJFd": nil,
	"ASTyfSima4LLAdDgoFGkgqoKowG1LZFDr9fAQrg7iaJZ": nil,
	"H8sMJSCQxfKiFTCfDR3DUMLPwcRbM61LGFJ8N4dK3WjS": nil,
	"GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE": nil,
	"5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9": nil,
	"2ojv9BAiHUrvsm9gxDe7fJSzbNZSJcxZvf8dqmWGHG8S": nil,
	"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD": nil,
	"2AQdpHJ2JpcEgPiATUXjQxA8QmafFegfQwSLWSprPicm": nil,
}

func isExchangeAddress(address string) bool {
	_, ok := exchangeAddresses[address]
	return ok
}

// signAndSendTx sends off a transaction and listens for completion
// it allows optional context to trigger fellow goroutines to stop sending / listening
// if one has already completed
func (b *Bot) signAndSendTx(tx *solana.Transaction, enableJito bool) (*solana.Signature, error) {
	txSig, err := tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if b.privateKey.PublicKey().Equals(key) {
				return &b.privateKey
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	startTs := time.Now()

	if enableJito {
		b.statusy("Sending transaction (Jito) " + txSig[0].String())

		_, err = b.jitoManager.jitoClient.BroadcastBundle([]*solana.Transaction{tx})
		if err != nil {
			return nil, err
		}

		if err = b.waitForTransactionComplete(txSig[0]); err != nil {
			return nil, err
		}

		latency := time.Since(startTs).Milliseconds()
		b.statusg(fmt.Sprintf("Sent transaction (Jito) %s with latency %d ms", txSig[0].String(), latency))

		return &txSig[0], nil
	}

	return b.sendTxVanilla(tx)
}

func (b *Bot) sendTxVanilla(tx *solana.Transaction) (*solana.Signature, error) {
	var txSig = tx.Signatures[0]
	var retries uint
	b.statusy("Sending Vanilla TX to Dedicated & Free RPCs: " + txSig.String())
	// send off tx with our dedicated rpc aka `b.rpcClient`
	go func() {
		if _, err := b.rpcClient.SendTransactionWithOpts(
			context.TODO(),
			tx,
			rpc.TransactionOpts{
				SkipPreflight: true,
				MaxRetries:    &retries,
			},
		); err != nil {
			fmt.Println("Error Sending Vanilla TX (Dedicated RPC)", err)
		}
	}()

	// use our free / alternate RPCs to send txs
	for _, rpcClient := range b.sendTxClients {
		go func(client *rpc.Client) {
			if err := b.sendOneVanillaTX(tx, client); err != nil {
				if strings.Contains(err.Error(), "429") {
					fmt.Println("Error Sending 1 Vanilla TX (Free RPC) (Ratelimited)")
				} else {
					fmt.Println("Error Sending 1 Vanilla TX (Free RPC)", err)
				}

			}
		}(rpcClient)
	}

	if err := b.waitForTransactionComplete(txSig); err != nil {
		return nil, err
	}

	return &txSig, nil
}

func (b *Bot) sendOneVanillaTX(tx *solana.Transaction, rpcClient *rpc.Client) error {
	var retries uint
	_, err := rpcClient.SendTransactionWithOpts(
		context.TODO(),
		tx,
		rpc.TransactionOpts{
			SkipPreflight: true,
			MaxRetries:    &retries,
		},
	)

	return err
}

func (b *Bot) fetchNLastTrans(numberSigs int, address string, optCtx ...context.Context) (jsonrpc.RPCResponses, error) {
	var ctx = context.TODO()
	if len(optCtx) > 0 {
		ctx = optCtx[0]
	}

	signatures, err := b.rpcClient.GetSignaturesForAddressWithOpts(
		ctx,
		solana.MustPublicKeyFromBase58(address),
		&rpc.GetSignaturesForAddressOpts{
			Commitment: rpc.CommitmentConfirmed,
			Limit:      &numberSigs,
		},
	)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline") {
			fmt.Println("Context timeout for", address)
			return nil, errors.New("context timeout")
		}

		log.Printf("Failed to fetch transactions for %s: %v\n", address, err)
		return nil, err
	}

	requests := make([]*jsonrpc.RPCRequest, len(signatures)) // Initializing an empty slice of pointers to RPCRequest structs

	for i, sig := range signatures {
		requests[i] = &jsonrpc.RPCRequest{
			JSONRPC: "2.0",
			ID:      i + 1,
			Method:  "getTransaction",
			Params:  []interface{}{sig.Signature, map[string]interface{}{"commitment": rpc.CommitmentConfirmed, "maxSupportedTransactionVersion": 0}},
		}
	}

	responses, err := b.jrpcClient.CallBatch(context.TODO(), requests)
	if err != nil {
		b.statusr(err)
		return nil, err
	}

	return responses, nil
}

// botHoldsTokens is a way for the bot to immediately check if we hold tokens
// does not represent whether we've bought yet or not.
func (c *Coin) botHoldsTokens() bool {
	if c.tokensHeld == nil {
		return false
	}

	heldTokensInt := c.tokensHeld.Int64()

	// TODO: do some checks to make sure no int overflow with this code
	// fmt.Println("Showing held tokens of", heldTokensInt)
	return heldTokensInt > 100
}

func (b *Bot) waitForTransactionComplete(sig solana.Signature) error {
	b.statusy("Waiting for transaction " + sig.String() + " to complete")

	signatureSubscription, err := b.wsClient.SignatureSubscribe(sig, rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}

	defer signatureSubscription.Unsubscribe()

	result, err := signatureSubscription.RecvWithTimeout(time.Duration(120) * time.Second)
	if err != nil {
		return err
	}

	if result.Value.Err != nil {
		return fmt.Errorf("Error in transaction: %v", result.Value.Err)
	}

	return nil
}

// lateToBuy compares the virtual sol reserves held in
// bonding curve compared to how much user bought of the coin,
// letting us know if we would be second buyer with current bonding curve
func (c *Coin) lateToBuy(bcd *BondingCurveData) bool {
	reservesLamports, _ := bcd.VirtualSolReserves.Float64()
	reservesSol := reservesLamports / float64(solana.LAMPORTS_PER_SOL)
	reservesLessCreatorSol := reservesSol - c.creatorPurchaseSol

	// consider data stale if someone in with more than 0.1
	// NOTE: we deduct 30 solana since that's already in bonding curve, provided by pump.fun
	return reservesLessCreatorSol-30 > 0.1
}
