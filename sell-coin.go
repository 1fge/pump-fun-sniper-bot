package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/1fge/pump-fun-sniper-bot/pump"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	cb "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
)

// SellCoinFast utilizes the fact that, unlike buying, we do not care if duplicate tx hit the chain
// if they do, we lose the priority fee, but ensure we are out of the position quickly. For this reason,
// we spam sell transactions every 400ms for a duration of 6 seconds, resulting in 15 sell tx
func (b *Bot) SellCoinFast(coin *Coin) {
	fmt.Println("Preparing to sell coin", coin.mintAddr.String())
	// send off sell requests separated by 400ms, wait for one to return
	// valid transaction, otherwise repeat (for 45 seconds at most)
	coin.isSellingCoin = true
	defer coin.setExitedSellCoinTrue()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6)
	defer cancel()

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	result := make(chan int, 1) // Buffered to ensure non-blocking send
	var sendVanilla = true

	// goroutine to send off sell tx every 400 until confirmed
	go func() {
		for {
			select {
			case <-ticker.C:
				// alternate between jito and vanilla each iteration, in case of no jito leader
				sendVanilla = !sendVanilla
				go b.sellCoinWrapper(coin, result, sendVanilla)
			case <-ctx.Done():
				return // Stop the ticker loop when context is cancelled
			}
		}
	}()

	// wait for first result to come back
	<-result
	time.Sleep(1 * time.Second)
}

func (b *Bot) sellCoinWrapper(coin *Coin, result chan int, sendVanilla bool) {
	sellSignature, err := b.sellCoin(coin, sendVanilla)
	if err != nil {
		if err != context.Canceled {
			if sellSignature != nil {
				b.statusr(fmt.Sprintf("Sell transaction %s failed: %s", sellSignature.String(), err))
			} else {
				b.statusr(fmt.Sprintf("Sell transaction failed: %s", err))
			}
		}

		return
	}

	if sellSignature == nil {
		fmt.Println("Sell signature is nil")
		return
	}

	result <- 1
}

func (b *Bot) sellCoin(coin *Coin, sendVanilla bool) (*solana.Signature, error) {
	if coin == nil {
		return nil, errNilCoin
	}

	sellInstruction := b.createSellInstruction(coin)
	culInst := cb.NewSetComputeUnitLimitInstruction(uint32(computeUnitLimits))
	cupInst := cb.NewSetComputeUnitPriceInstruction(b.feeMicroLamport)
	instructions := []solana.Instruction{cupInst.Build(), culInst.Build(), sellInstruction.Build()}

	// enable jito if it's jito leader and we do not force vanilla tx
	enableJito := b.jitoManager.isJitoLeader() && !sendVanilla
	if enableJito {
		coin.status("Jito leader, setting tip & removing priority fee inst")
		tipInst, err := b.jitoManager.generateTipInstruction()
		if err != nil {
			log.Fatal(err)
		}

		instructions = append(instructions, tipInst)

		// IMPORTANT: remove priority fee when we jito tip
		instructions = instructions[1:]
	}

	tx, err := b.createTransaction(instructions...)
	if err != nil {
		return nil, err
	}

	return b.signAndSendTx(tx, enableJito)
}

func (b *Bot) createSellInstruction(coin *Coin) *pump.Sell {
	// we want a minimum of 1 lamport, which ensures we should get filled at any price
	// as long as any of the 15 tx land
	minimumLamports := uint64(1)

	return pump.NewSellInstruction(
		coin.tokensHeld.Uint64(),
		minimumLamports,
		globalAddr,
		feeRecipient,
		coin.mintAddr,
		coin.tokenBondingCurve,
		coin.associatedBondingCurve,
		coin.associatedTokenAccount,
		b.privateKey.PublicKey(),
		solana.SystemProgramID,
		associatedtokenaccount.ProgramID,
		token.ProgramID,
		coin.eventAuthority,
		pumpProgramID,
	)
}

func (c *Coin) setExitedSellCoinTrue() {
	c.exitedSellCoin = true
}
