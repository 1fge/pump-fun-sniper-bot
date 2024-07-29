package geyser_client

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	_, filename, _, _ := runtime.Caller(0)
	godotenv.Load(filepath.Join(filepath.Dir(filename), "..", "..", "..", "jito-go", ".env"))
	os.Exit(m.Run())
}

func Test_GeyserClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rpcAddr, ok := os.LookupEnv("GEYSER_RPC")
	if !assert.True(t, ok, "getting GEYSER_RPC from .env") {
		t.FailNow()
	}

	if !assert.NotEqualf(t, "", rpcAddr, "GEYSER_RPC shouldn't be equal to [%s]", rpcAddr) {
		t.FailNow()
	}

	client, err := New(
		ctx,
		rpcAddr,
		nil,
	)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer client.GrpcConn.Close()

	// ion have a Geyser RPC although USDC program should work for both, if not lmk :)
	accounts := []string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"}
	programs := []string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"}

	t.Run("SubscribeBlockUpdates", func(t *testing.T) {
		sub, _, err := client.OnBlockUpdates(ctx)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		block := <-sub
		assert.NotNil(t, block.BlockUpdate.BlockHeight)
	})

	t.Run("SubscribePartialAccountUpdates", func(t *testing.T) {
		var sub proto.Geyser_SubscribePartialAccountUpdatesClient
		sub, err = client.SubscribePartialAccountUpdates()
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		var resp *proto.MaybePartialAccountUpdate
		resp, err = sub.Recv()
		assert.NoError(t, err)

		assert.NotNil(t, resp.GetHb())
	})

	t.Run("SubscribeAccountUpdates", func(t *testing.T) {
		var sub proto.Geyser_SubscribeAccountUpdatesClient
		sub, err = client.SubscribeAccountUpdates(accounts)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		var resp *proto.TimestampedAccountUpdate
		resp, err = sub.Recv()
		assert.NoError(t, err)

		assert.NotNil(t, resp.Ts)
		assert.NotNil(t, resp.AccountUpdate.TxSignature)
	})

	t.Run("SubscribeProgramUpdates", func(t *testing.T) {
		var sub proto.Geyser_SubscribeProgramUpdatesClient
		sub, err = client.SubscribeProgramUpdates(programs)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		var resp *proto.TimestampedAccountUpdate
		resp, err = sub.Recv()
		assert.NoError(t, err)

		assert.NotNil(t, resp.Ts)
		assert.NotNil(t, resp.AccountUpdate.TxSignature)
	})

	t.Run("SubscribeTransactionUpdates", func(t *testing.T) {
		var sub proto.Geyser_SubscribeTransactionUpdatesClient
		sub, err = client.SubscribeTransactionUpdates()
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		var resp *proto.TimestampedTransactionUpdate
		resp, err = sub.Recv()
		assert.NoError(t, err)

		assert.NotNil(t, resp.Ts)
		assert.NotNil(t, resp.Transaction)
	})

	t.Run("SubscribeSlotUpdates", func(t *testing.T) {
		var sub proto.Geyser_SubscribeSlotUpdatesClient
		sub, err = client.SubscribeSlotUpdates()
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		var resp *proto.TimestampedSlotUpdate
		resp, err = sub.Recv()
		assert.NotNil(t, err)

		assert.NotNil(t, resp.Ts)
		assert.NotNil(t, resp.SlotUpdate)
	})
}
