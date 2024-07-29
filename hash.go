package main

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
)

func (b *Bot) fetchBlockhashLoop() {
	go func() {
		for {
			err := b.fetchLatestBlockhash()
			if err != nil {
				b.statusr(err)
				continue
			}

			time.Sleep(400 * time.Millisecond)
		}
	}()
}

func (b *Bot) fetchLatestBlockhash() error {
	recent, err := b.rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}

	b.blockhash = &recent.Value.Blockhash
	return nil
}
