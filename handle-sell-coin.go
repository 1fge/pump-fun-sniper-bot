package main

import (
	"fmt"
	"time"
)

// HandleSellCoins iterates through our list of coins we've purchased,
// or intend to purchase, checks if they are stale (already sold / buy tx failed),
// or if they need to be sold, and handles both of those cases
func (b *Bot) HandleSellCoins() {
	for {
		coinsToSell := b.fetchCoinsToSell()

		for _, coin := range coinsToSell {
			go b.SellCoinFast(coin)
		}

		// check for coins we should sell each 100 ms
		time.Sleep(100 * time.Millisecond)
	}
}

// fetchCoinsToSell returns coins we should sell,
// but also deletes coins we no longer need to track
func (b *Bot) fetchCoinsToSell() []*Coin {
	var coinsToSell []*Coin

	b.pendingCoinsLock.Lock()
	defer b.pendingCoinsLock.Unlock()

	for mintAddr, coin := range b.pendingCoins {
		if coin == nil {
			continue
		}

		// if we exited BuyCoin & do not hold tokens, remove this coin
		if coin.exitedBuyCoin && !coin.botHoldsTokens() {
			fmt.Println("Deleting", coin.mintAddr.String(), "because exited buy but no hold")
			delete(b.pendingCoins, mintAddr)
		}

		// sold coins and stopped listening to creator, delete coin
		if coin.exitedSellCoin && coin.exitedCreatorListener {
			fmt.Println("Deleting", coin.mintAddr.String(), "because exited creator listener and sellCoins routine")
			delete(b.pendingCoins, mintAddr)
		}

		// we hold tokens & creator sold, must exit
		// make sure we are not already selling this coin
		if coin.botHoldsTokens() && coin.creatorSold && !coin.isSellingCoin {
			b.status(fmt.Sprintf("Selling %s: (decision=creator sold)", coin.mintAddr.String()))
			coinsToSell = append(coinsToSell, coin)
		}
	}

	return coinsToSell
}
