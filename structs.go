package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go/rpc/jsonrpc"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	_ "github.com/go-sql-driver/mysql"
)

var (
	errDBConnectionNil = errors.New("MySQL DB Connection Nil")

	pumpProgramID solana.PublicKey = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	globalAddr    solana.PublicKey = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	feeRecipient  solana.PublicKey = solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")
	rent          solana.PublicKey = solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
)

type Bot struct {
	rpcClient     *rpc.Client
	jrpcClient    rpc.JSONRPCClient
	sendTxClients []*rpc.Client

	wsClient     *ws.Client
	privateKey   solana.PrivateKey
	dbConnection *sql.DB

	feeMicroLamport  uint64
	buyAmountLamport uint64 // amount of coins we buy for each coin (in lamports)

	pendingCoins     map[string]*Coin // coins which we will attempt to buy, but have yet to be purchased
	pendingCoinsLock sync.Mutex
	coinsToBuy       chan *Coin
	coinsToSell      chan string

	// skipATALookup skips looking up if the ATA exists. Useful for debugging & attempting to purchase coins we already have owned.
	// in prod, should always be set to `true` since we should never have ATA for new coins.
	skipATALookup bool

	blockhash   *solana.Hash
	jitoManager *JitoManager
}

func (b *Bot) status(msg interface{}) {
	log.Println("Bot", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusy(msg interface{}) {
	log.Println("Bot (Y)", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusg(msg interface{}) {
	log.Println("Bot (G)", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusr(msg interface{}) {
	log.Println("Bot (R)", fmt.Sprintf("%v", msg))
}

type Coin struct {
	pickupTime time.Time // used to make sure duration / timings are good

	mintAddr               solana.PublicKey
	tokenBondingCurve      solana.PublicKey
	associatedBondingCurve solana.PublicKey
	eventAuthority         solana.PublicKey

	creator            solana.PublicKey
	creatorATA         solana.PublicKey
	creatorPurchased   bool
	creatorPurchaseSol float64 // actual solana amount of buy, not lamports

	// our values related to the coin once we buy / decide to buy, and afterwards
	creatorSold  bool // has creator sold?
	botPurchased bool // separate bool.

	exitedBuyCoin         bool // trigger to notify that we have finished all buy ops
	exitedSellCoin        bool // trigger to notify that we have exited sell code routine
	exitedCreatorListener bool // trigger to notify that we stopped listening to creator sell

	isSellingCoin bool // lets program know that we are already in the process of selling coin to avoid dup sell

	associatedTokenAccount solana.PublicKey // our wallet's ata for this coin
	tokensHeld             *big.Int

	buyPrice                uint64
	buyTransactionSignature *solana.Signature
}

func (c *Coin) status(msg interface{}) {
	log.Println(c.mintAddr.String(), fmt.Sprintf("%v", msg))
}

func proxiedClient(endpoint string) jsonrpc.RPCClient {
	u, _ := url.Parse(proxyURL)
	opts := &jsonrpc.RPCClientOpts{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(u),
			},
		},
	}

	return jsonrpc.NewClientWithOpts(endpoint, opts)
}

// NewBot creates a new bot struct that we use to buy & sell coins
func NewBot(rpcURL, wsURL, privateKey string, dbConnection *sql.DB, buySol float64, feeMicroLamport uint64) (*Bot, error) {
	var rpcClient *rpc.Client
	var jrpcClient rpc.JSONRPCClient

	if shouldProxy {
		rpcClient = rpc.NewWithCustomRPCClient(proxiedClient(rpcURL))
		jrpcClient = proxiedClient(rpcURL)
	} else {
		rpcClient = rpc.New(rpcURL)
		jrpcClient = rpc.NewWithRateLimit(rpcURL, 500)
	}

	wsClient, err := ws.Connect(context.Background(), wsURL)
	if err != nil {
		fmt.Println("ws connection err", err)
		return nil, err
	}

	if dbConnection == nil {
		return nil, errDBConnectionNil
	}

	botPrivKey, err := solana.PrivateKeyFromBase58(privateKey)
	if err != nil {
		return nil, err
	}

	buySolToLamport := buySol * float64(solana.LAMPORTS_PER_SOL)

	jitoManager, err := newJitoManager(rpcClient, botPrivKey)
	if err != nil {
		return nil, err
	}

	var sendTxClients []*rpc.Client
	for _, txRPC := range sendTxRPCs {
		sendTxClients = append(sendTxClients, rpc.New(txRPC))
	}

	b := &Bot{
		rpcClient:     rpcClient,
		jrpcClient:    jrpcClient,
		wsClient:      wsClient,
		sendTxClients: sendTxClients,

		privateKey:       botPrivKey,
		dbConnection:     dbConnection,
		buyAmountLamport: uint64(buySolToLamport),
		feeMicroLamport:  feeMicroLamport,

		jitoManager: jitoManager,

		pendingCoins:     make(map[string]*Coin),
		pendingCoinsLock: sync.Mutex{},
		coinsToBuy:       make(chan *Coin),
		coinsToSell:      make(chan string),
	}

	b.fetchBlockhashLoop()
	return b, nil
}

func (b *Bot) beginJito() error {
	if err := b.jitoManager.start(); err != nil {
		return err
	}

	return nil
}
