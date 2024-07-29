package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/1fge/pump-fun-sniper-bot/pump"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	cb "github.com/gagliardetto/solana-go/programs/compute-budget"
)

var (
	// compute units never seem to get close to exceeding 70,000 so no need to set higher
	computeUnitLimits uint32 = 70000
	errNilCoin               = errors.New("Nil Coin")
	errLateToCoin            = errors.New("Coin has multiple buyers (BCD)")
)

// BuyCoin handles the code for purchasing a single coin, updating program
// state depending on the success of the purchase or not
func (b *Bot) BuyCoin(coin *Coin) error {
	var shouldCreateATA bool
	defer coin.setExitedBuyCoinTrue()

	var instructions []solana.Instruction

	if coin == nil {
		return errNilCoin
	}

	// coin not nil, display buy status
	buyStatus := fmt.Sprintf("Attempting to buy %s (%v)", coin.mintAddr.String(), time.Since(coin.pickupTime))
	b.status(buyStatus)

	ataAddress, err := b.calculateATAAddress(coin)
	if err != nil {
		return err
	}

	if b.skipATALookup {
		shouldCreateATA = true
	} else {
		coin.status("Checking associated token: " + ataAddress.String())
		shouldCreateATA, err = b.shouldCreateATA(ataAddress)
		if err != nil {
			return err
		}
	}

	coin.status("Fetching bonding curve")
	bcd, err := b.fetchBondingCurve(coin.tokenBondingCurve)
	if err != nil {
		return err
	}

	// protect us from stale data, bad buy price
	// by checking if someone else has already purchased through BCD
	coin.status(fmt.Sprintf("Fetched bonding curve, (%s)", bcd.String()))
	if coin.lateToBuy(bcd) {
		return errLateToCoin
	}

	// determine num tokens to buy based on sol buy amount,
	// set very low slippage tolerance (2% max slippage) so we ensure we
	// enter in position as second buyer
	coin.buyPrice = b.buyAmountLamport
	tokensToBuy := calculateBuyQuote(b.buyAmountLamport, bcd, 0.98)
	buyInstruction := b.createBuyInstruction(tokensToBuy, coin, *ataAddress)

	// create priority fee instructions
	culInst := cb.NewSetComputeUnitLimitInstruction(uint32(computeUnitLimits))
	cupInst := cb.NewSetComputeUnitPriceInstruction(b.feeMicroLamport)

	if shouldCreateATA {
		_, createAtaInstruction, err := b.createATA(coin)
		if err != nil {
			return err
		}
		instructions = []solana.Instruction{cupInst.Build(), culInst.Build(), createAtaInstruction, buyInstruction.Build()}
	} else {
		instructions = []solana.Instruction{cupInst.Build(), culInst.Build(), buyInstruction.Build()}
	}

	enableJito := b.jitoManager.isJitoLeader()
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

	coin.status("Creating transaction")
	tx, err := b.createTransaction(instructions...)
	if err != nil {
		return err
	}

	coin.status("Sending transaction")
	if _, err = b.signAndSendTx(tx, enableJito); err != nil {
		if !strings.Contains(err.Error(), "transaction has already been processed") {
			return err
		}
	}

	// notify chans we have purchased & set amount of owned tokens
	coin.botPurchased = true
	coin.tokensHeld = tokensToBuy
	coin.associatedTokenAccount = *ataAddress
	coin.buyTransactionSignature = &tx.Signatures[0]

	return nil
}

func (c *Coin) setExitedBuyCoinTrue() {
	c.exitedBuyCoin = true
}

// calculateATAAddress calculates the associated token account address for the bot's public key and the coin's mint address.
// The address is a deterministic address based on the public key and the mint address.
func (b *Bot) calculateATAAddress(coin *Coin) (*solana.PublicKey, error) {
	coin.status("Calculating associated token address")

	ata, _, err := solana.FindAssociatedTokenAddress(b.privateKey.PublicKey(), coin.mintAddr)
	if err != nil {
		return nil, err
	}

	return &ata, nil
}

// shouldCreateATA checks if the associated token account for the mint and our bot's public key exists.
func (b *Bot) shouldCreateATA(ataAddress *solana.PublicKey) (bool, error) {
	_, err := b.rpcClient.GetAccountInfo(context.TODO(), *ataAddress)
	if err == nil {
		return false, nil
	}

	return true, nil
}

// createATA creates associated token account for the mint and our bot's public key.
// it also validateAndBuilds the instruction for creating the new address
// NOTE: we always assume we do not have an ATA for the coin since we never buy twice
func (b *Bot) createATA(coin *Coin) (solana.PublicKey, *associatedtokenaccount.Instruction, error) {
	var botPubKey solana.PublicKey = b.privateKey.PublicKey()
	var defaultPubKey solana.PublicKey = solana.PublicKey{}

	ata, _, err := solana.FindAssociatedTokenAddress(botPubKey, coin.mintAddr)
	if err != nil {
		return defaultPubKey, nil, err
	}

	// Create the associated token account instruction
	createATAInstruction, err := associatedtokenaccount.NewCreateInstruction(
		botPubKey,     // Payer
		botPubKey,     // Wallet owner
		coin.mintAddr, // Token mint
	).ValidateAndBuild()
	if err != nil {
		return defaultPubKey, nil, err
	}

	return ata, createATAInstruction, nil
}

func (b *Bot) createBuyInstruction(tokensToBuy *big.Int, coin *Coin, ata solana.PublicKey) *pump.Buy {
	return pump.NewBuyInstruction(
		tokensToBuy.Uint64(),
		b.buyAmountLamport,
		globalAddr,
		feeRecipient,
		coin.mintAddr,
		coin.tokenBondingCurve,
		coin.associatedBondingCurve,
		ata,
		b.privateKey.PublicKey(),
		solana.SystemProgramID,
		solana.TokenProgramID,
		rent,
		coin.eventAuthority,
		pumpProgramID,
	)
}

func (b *Bot) createTransaction(instructions ...solana.Instruction) (*solana.Transaction, error) {
	// Prepare the transaction with both the associated token account creation and the buy instructions
	return solana.NewTransaction(
		instructions,
		*b.blockhash,
		solana.TransactionPayer(b.privateKey.PublicKey()),
	)
}
