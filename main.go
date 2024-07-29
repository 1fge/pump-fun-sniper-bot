package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	// set up to be run on same machine as dedicated RPC
	// can be swapped out to separate RPC url
	rpcURL   = "http://127.0.0.1:8799"
	wsURL    = "ws://127.0.0.1:8800"
	proxyURL = ""

	sendTxRPCs = []string{
		// insert public RPCs / alernate RPCs here to increase likelihood of tx landing
	}

	shouldProxy = strings.Contains(os.Getenv("PROXY_URL"), "http")
)

func loadPrivateKey() (string, error) {
	if err := godotenv.Load(); err != nil {
		return "", err
	}

	return os.Getenv("PRIVATE_KEY"), nil
}

func main() {
	db, err := sql.Open("mysql", "root:XXXXXX!@/CoinTrades")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	privateKey, err := loadPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	proxyURL = os.Getenv("PROXY_URL")

	// purchase coins with 0.05 solana, priority fee of 200000 microlamp
	bot, err := NewBot(rpcURL, wsURL, privateKey, db, 0.05, 200000)
	if err != nil {
		log.Fatal(err)
	}

	bot.skipATALookup = true

	go bot.HandleNewMints()
	go bot.HandleBuyCoins()
	go bot.HandleSellCoins()

	if err := bot.beginJito(); err != nil {
		log.Fatal("Error Starting Jito", err)
	}

	select {}
}
