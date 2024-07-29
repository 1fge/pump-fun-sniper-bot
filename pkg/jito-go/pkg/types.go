package pkg

import (
	"github.com/gagliardetto/solana-go"
	"time"
)

type Keypair struct {
	PublicKey  solana.PublicKey
	PrivateKey solana.PrivateKey
}

type Platform string

var (
	SolanaFM       Platform = "https://solana.fm/tx/"
	Solscan        Platform = "https://solscan.io/tx/"
	SolanaExplorer Platform = "https://explorer.solana.com/tx/"
	SolanaBeach    Platform = "https://solanabeach.io/transaction/"
	XRAY           Platform = "https://xray.helius.xyz/tx/"
)

const tipStreamURL = "ws://bundles-api-rest.jito.wtf/api/v1/bundles/tip_stream"

type TipStreamInfo struct {
	Time                        time.Time `json:"time"`
	LandedTips25ThPercentile    float64   `json:"landed_tips_25th_percentile"`
	LandedTips50ThPercentile    float64   `json:"landed_tips_50th_percentile"`
	LandedTips75ThPercentile    float64   `json:"landed_tips_75th_percentile"`
	LandedTips95ThPercentile    float64   `json:"landed_tips_95th_percentile"`
	LandedTips99ThPercentile    float64   `json:"landed_tips_99th_percentile"`
	EmaLandedTips50ThPercentile float64   `json:"ema_landed_tips_50th_percentile"`
}
