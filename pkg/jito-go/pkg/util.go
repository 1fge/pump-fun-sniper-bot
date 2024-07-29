package pkg

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gorilla/websocket"
)

// ExtractSigFromTx extracts the transaction's signature.
func ExtractSigFromTx(tx *solana.Transaction) solana.Signature {
	return tx.Signatures[0]
}

func BatchExtractSigFromTx(txns []*solana.Transaction) []solana.Signature {
	sigs := make([]solana.Signature, 0, len(txns))
	for _, tx := range txns {
		sigs = append(sigs, tx.Signatures[0])
	}
	return sigs
}

func BuildTransactionLinks(txns []solana.Signature, platform Platform) []string {
	txs := make([]string, 0, len(txns))
	for _, tx := range txns {
		txs = append(txs, fmt.Sprintf("%s%s", platform, tx.String()))
	}
	return txs
}

// NewKeyPair creates a Keypair from a private key.
func NewKeyPair(privateKey solana.PrivateKey) *Keypair {
	return &Keypair{PrivateKey: privateKey, PublicKey: privateKey.PublicKey()}
}

// SubscribeTipStream establishes a connection to the Jito websocket and receives TipStreamInfo.
func SubscribeTipStream(ctx context.Context) (<-chan *TipStreamInfo, <-chan error, error) {
	dialer := websocket.Dialer{}
	ch := make(chan *TipStreamInfo)
	chErr := make(chan error)

	conn, _, err := dialer.Dial(tipStreamURL, nil)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var r []TipStreamInfo
				if err = conn.ReadJSON(&r); err != nil {
					fmt.Println("err hson", err)
					chErr <- err
					time.Sleep(10 * time.Millisecond)
					continue
				}

				ch <- &r[0]
			}
		}
	}()

	return ch, chErr, nil
}

// GenerateKeypair creates a new Solana Keypair.
func GenerateKeypair() *Keypair {
	wallet := solana.NewWallet()
	return &Keypair{PublicKey: wallet.PublicKey(), PrivateKey: wallet.PrivateKey}
}

func LamportsToSol(lamports *big.Float) *big.Float {
	return new(big.Float).Quo(lamports, new(big.Float).SetUint64(solana.LAMPORTS_PER_SOL))
}

// StrSliceToByteSlice converts a string array to a byte array.
func StrSliceToByteSlice(s []string) [][]byte {
	byteSlice := make([][]byte, 0, len(s))
	for _, b := range s {
		byteSlice = append(byteSlice, []byte(b))
	}
	return byteSlice
}
