package jito_go

import (
	"github.com/gagliardetto/solana-go"
)

var MainnetTipPaymentProgram = solana.MustPublicKeyFromBase58("T1pyyaTNZsKv2WcRAB8oVnk93mLJw2XzjtVYqCsaHqt")
var MainnetTipDistributionProgram = solana.MustPublicKeyFromBase58("4R3gSG8BpU4t19KYj8CfnbtRpnT8gtk4dvTHxVRwc2r7")
var MainnetMerkleUploadAuthorityProgram = solana.MustPublicKeyFromBase58("GZctHpWXmsZC1YHACTGGcHhYxjdRqQvTpYkb9LMvxDib")

var MainnetTipAccounts = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5"),
	solana.MustPublicKeyFromBase58("HFqU5x63VTqvQss8hp11i4wVV8bD44PvwucfZ2bU7gRe"),
	solana.MustPublicKeyFromBase58("Cw8CFyM9FkoMi7K7Crf6HNQqf4uEMzpKw6QNghXLvLkY"),
	solana.MustPublicKeyFromBase58("ADaUMid9yfUytqMBgopwjb2DTLSokTSzL1zt6iGPaS49"),
	solana.MustPublicKeyFromBase58("DfXygSm4jCyNCybVYYK6DwvWqjKee8pbDmJGcLWNDXjh"),
	solana.MustPublicKeyFromBase58("ADuUkR4vqLUMWXxW9gh6D6L8pMSawimctcNZ5pGwDcEt"),
	solana.MustPublicKeyFromBase58("DttWaMuVvTiduZRnguLF7jNxTgiMBZ1hyAumKUiL2KRL"),
	solana.MustPublicKeyFromBase58("3AVi9Tg9Uo68tJfuvoKvqKNWKkC5wPdSSdeBnizKZ6jT"),
}

var TestnetTipPaymentProgram = solana.MustPublicKeyFromBase58("DCN82qDxJAQuSqHhv2BJuAgi41SPeKZB5ioBCTMNDrCC")
var TestnetTipDistributionProgram = solana.MustPublicKeyFromBase58("F2Zu7QZiTYUhPd7u9ukRVwxh7B71oA3NMJcHuCHc29P2")
var TestnetMerkleUploadAuthorityProgram = solana.MustPublicKeyFromBase58("GZctHpWXmsZC1YHACTGGcHhYxjdRqQvTpYkb9LMvxDib")

var TestnetTipAccounts = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("B1mrQSpdeMU9gCvkJ6VsXVVoYjRGkNA7TtjMyqxrhecH"),
	solana.MustPublicKeyFromBase58("aTtUk2DHgLhKZRDjePq6eiHRKC1XXFMBiSUfQ2JNDbN"),
	solana.MustPublicKeyFromBase58("E2eSqe33tuhAHKTrwky5uEjaVqnb2T9ns6nHHUrN8588"),
	solana.MustPublicKeyFromBase58("4xgEmT58RwTNsF5xm2RMYCnR1EVukdK8a1i2qFjnJFu3"),
	solana.MustPublicKeyFromBase58("EoW3SUQap7ZeynXQ2QJ847aerhxbPVr843uMeTfc9dxM"),
	solana.MustPublicKeyFromBase58("ARTtviJkLLt6cHGQDydfo1Wyk6M4VGZdKZ2ZhdnJL336"),
	solana.MustPublicKeyFromBase58("9n3d1K5YD2vECAbRFhFFGYNNjiXtHXJWn9F31t89vsAV"),
	solana.MustPublicKeyFromBase58("9ttgPBBhRYFuQccdR1DSnb7hydsWANoDsV3P9kaGMCEh"),
}

type JitoEndpointInfo struct {
	Region            string
	BlockEngineURL    string
	RelayerURL        string
	ShredReceiverAddr string
	Ntp               string
}

var JitoEndpoints = map[string]JitoEndpointInfo{
	"AMS": {
		Region:            "amsterdam",
		BlockEngineURL:    "amsterdam.mainnet.block-engine.jito.wtf:443",
		RelayerURL:        "amsterdam.mainnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "74.118.140.240:1002",
		Ntp:               "ntp.amsterdam.jito.wtf",
	},
	"FFM": {
		Region:            "frankfurt",
		BlockEngineURL:    "frankfurt.mainnet.block-engine.jito.wtf:443",
		RelayerURL:        "frankfurt.mainnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "145.40.93.84:1002",
		Ntp:               "ntp.frankfurt.jito.wtf",
	},
	"NY": {
		Region:            "ny",
		BlockEngineURL:    "ny.mainnet.block-engine.jito.wtf:443",
		RelayerURL:        "ny.mainnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "141.98.216.96:1002",
		Ntp:               "ntp.dallas.jito.wtf",
	},
	"TKY": {
		Region:            "tokyo",
		BlockEngineURL:    "tokyo.mainnet.block-engine.jito.wtf:443",
		RelayerURL:        "tokyo.mainnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "202.8.9.160:1002",
		Ntp:               "ntp.tokyo.jito.wtf",
	},
	"BigD-TESTNET": {
		BlockEngineURL:    "dallas.testnet.block-engine.jito.wtf:443",
		RelayerURL:        "dallas.testnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "147.28.154.132:1002",
		Ntp:               "ntp.dallas.jito.wtf",
	},
	"NY-TESTNET": {
		BlockEngineURL:    "ny.testnet.block-engine.jito.wtf:443",
		RelayerURL:        "nyc.testnet.relayer.jito.wtf:8100",
		ShredReceiverAddr: "141.98.216.97:1002",
		Ntp:               "ntp.dallas.jito.wtf", // Dallas NTP is suitable for NY connections (from jito's doc)
	},
}

var Amsterdam = JitoEndpoints["AMS"]
var Frankfurt = JitoEndpoints["FFM"]
var NewYork = JitoEndpoints["NY"]
var Tokyo = JitoEndpoints["TKY"]
var TestnetDallas = JitoEndpoints["BigD-TESTNET"]
var TestnetNewYork = JitoEndpoints["NY-TESTNET"]

const JitoMainnet = "mainnet.rpc.jito.wtf"
