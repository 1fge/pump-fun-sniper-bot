package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"

	pump "github.com/1fge/pump-fun-sniper-bot/pump"
)

type instPair struct {
	tx   *solana.Transaction
	meta *rpc.TransactionMeta
}

// HandleBuyCoins is run as a goroutine which keeps waiting for
// new coins to enter the `coinsToBuy` channel
// we will start the buying process async and update our coins map
// with the coin at the same time
func (b *Bot) HandleBuyCoins() {
	for coin := range b.coinsToBuy {
		go b.purchaseCoin(coin)
	}
}

func (b *Bot) purchaseCoin(coin *Coin) {
	if coin == nil {
		return
	}

	// add in new coin to pending coins
	b.addNewPendingCoin(coin)

	// immediately start listening for a creator sell
	go b.listenCreatorSell(coin)

	if err := b.BuyCoin(coin); err != nil {
		b.statusy("Error Buying Coin: " + err.Error())
		return
	}

	fmt.Println("Purchased Coin", coin.mintAddr.String())
}

func (b *Bot) addNewPendingCoin(coin *Coin) {
	b.pendingCoinsLock.Lock()
	defer b.pendingCoinsLock.Unlock()

	mintAddr := coin.mintAddr.String()
	b.pendingCoins[mintAddr] = coin
}

func (b *Bot) listenCreatorSell(coin *Coin) {
	// subscribe to our creator ATA with our ws client
	defer coin.setExitedCreatorListenerTrue()

	sub, err := b.wsClient.AccountSubscribe(coin.creatorATA, rpc.CommitmentConfirmed)
	if err != nil {
		log.Printf("Failed to subscribe to logs: %v", err)
		b.setCreatorSold(coin)
		return
	}

	defer sub.Unsubscribe()

	for {
		// act as signal to fetch latest transactions
		_, err := sub.Recv()
		if err != nil {
			log.Printf("Error receiving AccountSubscribe: %v\n", err)
			b.setCreatorSold(coin)
			return
		}

		// if we exited BuyCoin & didn't purchase, exit listener
		// alternatively, if we purchased but don't hold tokens any longer, exit listener
		if (coin.exitedBuyCoin && !coin.botPurchased) || (coin.botPurchased && !coin.botHoldsTokens()) {
			fmt.Println("No buy recorded or bot already sold tokens, stopping listener")
			return
		}

		// variable which allows us to see if we managed to check ATA activity
		// if we didn't mark as sold since we cannot trust data we have

		// check 10 times to allow catching up with new data / timeouts, if RPC experiencing issues
		for checkAttempts := 0; checkAttempts < 10; checkAttempts++ {
			instPairs, err := b.fetchCreatorATATrans(coin)
			if err != nil {
				log.Printf("Error Fetching Creator Transactions, continuing to next loop: " + err.Error() + "\n")
				continue
			}

			if b.isSellOrTransfer(instPairs, coin) {
				b.status(fmt.Sprintf("Detected Sale / Transfer, Marking as sold %s", coin.mintAddr.String()))
				b.setCreatorSold(coin)
				return
			}

			time.Sleep(200 * time.Millisecond)
		}

		fmt.Println("Activity for ATA", coin.creatorATA.String(), "was not sell/transfer")
	}
}

func (c *Coin) setExitedCreatorListenerTrue() {
	c.exitedCreatorListener = true
}

// update that creator has sold (used on actual sell / transfer & err)
func (b *Bot) setCreatorSold(coin *Coin) {
	b.pendingCoinsLock.Lock()
	defer b.pendingCoinsLock.Unlock()

	mintAddr := coin.mintAddr.String()
	if _, ok := b.pendingCoins[mintAddr]; ok {
		b.pendingCoins[mintAddr].creatorSold = true
	}
}

// fetchCreatorATATrans pulls latest 3 transactions after we detect change
// to a creatorATA account. It returns instruction pair containing tx data, along with
// meta, so we can fetch innerinstructions for the tx
func (b *Bot) fetchCreatorATATrans(coin *Coin) ([]instPair, error) {
	var instPairs []instPair

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*900)
	defer cancel()

	latestTransResps, err := b.fetchNLastTrans(3, coin.creatorATA.String(), ctx)
	if err != nil {
		return nil, err
	}

	for _, resp := range latestTransResps {
		var transResult *rpc.GetTransactionResult = &rpc.GetTransactionResult{}
		if err := resp.GetObject(&transResult); err != nil {
			continue
		}

		if transResult == nil || transResult.Transaction == nil {
			continue
		}

		tx, err := transResult.Transaction.GetTransaction()
		if err != nil {
			continue
		}

		meta := transResult.Meta
		instPairs = append(instPairs, instPair{tx: tx, meta: meta})
	}

	return instPairs, nil
}

func (b *Bot) isSellOrTransfer(instPairs []instPair, coin *Coin) bool {
	// immediately check for a sell
	for _, instPair := range instPairs {
		if detectTransfer(instPair, coin) {
			return true
		}
	}

	return detectSell(instPairs)
}

// detectSell uses the instruction pairs from the creator ATA detected tx
// to see if a sell was detected in those instructions
func detectSell(instPairs []instPair) bool {
	for _, instPair := range instPairs {
		for _, instruction := range instPair.tx.Message.Instructions {
			// Find the accounts of this instruction:
			accounts, err := instruction.ResolveInstructionAccounts(&instPair.tx.Message)
			if err != nil {
				continue
			}

			instr, err := pump.DecodeInstruction(accounts, instruction.Data)
			if err != nil {
				continue
			}

			data, err := instr.Data()
			if err != nil || len(data) < 8 {
				continue
			}

			typeID := data[0:8]

			for k, v := range pumpIDs {
				if k.Equal(typeID) {
					switch v.name {
					case "sell":
						fmt.Println("*** Found a sell in the decodedInstructions")
						return true
					}
				}
			}
		}
	}

	return false
}

func detectTransfer(pair instPair, coin *Coin) bool {
	if pair.meta == nil || len(pair.meta.InnerInstructions) == 0 {
		return false
	}

	for _, inst := range pair.meta.InnerInstructions {
		for _, innerInst := range inst.Instructions {
			progKey, err := pair.tx.ResolveProgramIDIndex(innerInst.ProgramIDIndex)
			if err != nil {
				continue
			}

			if !progKey.Equals(token.ProgramID) {
				continue
			}

			accounts, err := innerInst.ResolveInstructionAccounts(&pair.tx.Message)
			if err != nil {
				continue
			}

			decodedInstruction, err := token.DecodeInstruction(accounts, innerInst.Data)
			if err != nil {
				continue
			}

			// TODO: See if this is actually necessary. Would burn appear as transfer?
			// if _, ok := decodedInstruction.Impl.(*token.Burn); ok {
			// 	fmt.Println("User burned tokens")
			// 	return false
			// }

			// Check for a transfer instruction
			if transferInst, ok := decodedInstruction.Impl.(*token.Transfer); ok {
				sender := transferInst.GetSourceAccount().PublicKey.String()
				if sender == coin.creatorATA.String() {
					return true
				}
			}

		}
	}

	return false
}
