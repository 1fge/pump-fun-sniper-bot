package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	pump "github.com/1fge/pump-fun-sniper-bot/pump"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

type pumpInstr struct {
	name        string
	impl        reflect.Type
	programName string
	isPump      bool
}

var (
	errBadCreateInstruction = errors.New("Bad `Create` Instruction")
	errNoCreatorATA         = errors.New("No Creator ATA")
	errCreatingNewCoin      = errors.New("Unknown Error Creating New Coin")
	errNoCreatorBuy         = errors.New("No Creator Buy Found")
)

var pumpIDs = map[bin.TypeID]*pumpInstr{
	pump.Instruction_Create:     &pumpInstr{programName: "Pump", name: "create", impl: reflect.TypeOf(pump.Create{}), isPump: true},
	pump.Instruction_Buy:        &pumpInstr{programName: "Pump", name: "buy", impl: reflect.TypeOf(pump.Buy{}), isPump: true},
	pump.Instruction_Sell:       &pumpInstr{programName: "Pump", name: "sell", impl: reflect.TypeOf(pump.Sell{}), isPump: true},
	pump.Instruction_Withdraw:   &pumpInstr{programName: "Pump", name: "withdraw", impl: reflect.TypeOf(pump.Withdraw{}), isPump: true},
	pump.Instruction_Initialize: &pumpInstr{programName: "Pump", name: "initialize", impl: reflect.TypeOf(pump.Initialize{}), isPump: true},
	pump.Instruction_SetParams:  &pumpInstr{programName: "Pump", name: "set_params", impl: reflect.TypeOf(pump.SetParams{}), isPump: true},

	bin.TypeID([8]byte{2, 0, 0, 0, 224, 147, 4, 0}): &pumpInstr{programName: "System Program", name: "Transfer", impl: nil, isPump: false},
	bin.TypeID([8]byte{3, 160, 134, 1, 0, 0, 0, 0}): &pumpInstr{programName: "Compute Budget", name: "SetComputeUnitPrice", impl: nil, isPump: false},
	bin.TypeID([8]byte{2, 160, 134, 1, 0, 7, 2, 0}): &pumpInstr{programName: "Compute Budget", name: "SetComputeUnitLimit", impl: nil, isPump: false},
}

// HandleNewMints runs as goroutine, subscribing to logs for pump program
// if we detect a coin we should buy, it's passed off to buy / sell handler
func (b *Bot) HandleNewMints() {
	fmt.Println("Listening for new mints...")

	sub, err := b.wsClient.LogsSubscribeMentions(pumpProgramID, rpc.CommitmentConfirmed)
	if err != nil {
		log.Fatalf("Failed to subscribe to pump program logs: %v", err)
	}
	defer sub.Unsubscribe()

	for {
		msg, err := sub.Recv()
		if err != nil {
			log.Printf("Error receiving log: %v\n", err)
			continue
		}

		// Analyze the logs to detect mint operations
		for _, logEntry := range msg.Value.Logs {
			if !isMintLog(logEntry) {
				continue
			}

			b.status("Detected Mint (" + msg.Value.Signature.String() + ")")
			go b.checkAndSignalBuyCoin(msg.Value.Signature)
		}
	}
}

// check if new coin should be bought & handle async
func (b *Bot) checkAndSignalBuyCoin(mintSig solana.Signature) {
	start := time.Now()
	newCoin, err := b.fetchMintDetails(mintSig)
	if err != nil {
		log.Print(err)
		return
	}

	if !b.shouldBuyCoin(newCoin) {
		return
	}

	if time.Since(start) > 2*time.Second {
		b.status(fmt.Sprintf("Skipping %s (detail fetch took too long)", newCoin.mintAddr.String()))
		return
	}

	newCoin.pickupTime = start
	b.coinsToBuy <- newCoin
}

// fetchMintDetails returns data on the coin like addresses associated with BC,
// associated bonding curve, and creator information like how many coins they purchased
func (b *Bot) fetchMintDetails(sig solana.Signature) (*Coin, error) {
	version := uint64(0)
	tx, err := b.rpcClient.GetTransaction(
		context.Background(),
		sig,
		&rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &version,
			Encoding:                       solana.EncodingBase64,
			Commitment:                     rpc.CommitmentConfirmed,
		},
	)

	if err != nil {
		return nil, errors.New("Failed to fetch mint transaction: " + err.Error())
	}

	decodedTx, err := tx.Transaction.GetTransaction()
	if err != nil {
		return nil, err
	}

	newCoin, err := fetchNewCoin(decodedTx)
	if err != nil {
		return nil, err
	}

	if err := newCoin.fetchCreatorBuy(decodedTx); err != nil {
		return nil, err
	}

	return newCoin, nil
}

func fetchNewCoin(decodedTx *solana.Transaction) (*Coin, error) {
	for _, instruction := range decodedTx.Message.Instructions {
		// Find the accounts of this instruction:
		accounts, err := instruction.ResolveInstructionAccounts(&decodedTx.Message)
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
				x := reflect.New(v.impl).Interface()
				switch v.name {
				case "create":
					p := x.(*pump.Create)
					p.AccountMetaSlice = accounts

					if err := p.UnmarshalWithDecoder(bin.NewBorshDecoder(data[8:])); err != nil {
						return nil, fmt.Errorf("error Decoding `Create`:%v", err)
					}

					return newCoinFromCreateInst(p)
				}
			}
		}
	}

	return nil, errCreatingNewCoin
}

func newCoinFromCreateInst(inst *pump.Create) (*Coin, error) {
	mintAddr := inst.GetMintAccount()
	bondingCurve := inst.GetBondingCurveAccount()
	associatedBondingCurve := inst.GetAssociatedBondingCurveAccount()
	eventAuthority := inst.GetEventAuthorityAccount()
	creatorAddr := inst.GetUserAccount()

	if creatorAddr == nil || mintAddr == nil || bondingCurve == nil || associatedBondingCurve == nil || eventAuthority == nil {
		return nil, errBadCreateInstruction
	}

	return &Coin{
		mintAddr:               mintAddr.PublicKey,
		tokenBondingCurve:      bondingCurve.PublicKey,
		associatedBondingCurve: associatedBondingCurve.PublicKey,
		eventAuthority:         eventAuthority.PublicKey,
		creator:                creatorAddr.PublicKey,
	}, nil
}

// fetchCreatorBuy detects creator buy from mint inst and:
// fetches buy amount (if any)
// sets creator ATA address

func (c *Coin) fetchCreatorBuy(decodedTx *solana.Transaction) error {
	for _, instruction := range decodedTx.Message.Instructions {
		// Find the accounts of this instruction:
		accounts, err := instruction.ResolveInstructionAccounts(&decodedTx.Message)
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
				x := reflect.New(v.impl).Interface()
				switch v.name {
				case "buy":
					p := x.(*pump.Buy)
					p.AccountMetaSlice = accounts
					if err := p.UnmarshalWithDecoder(bin.NewBorshDecoder(data[8:])); err != nil {
						return fmt.Errorf("err unmarshalling buy data: %s", err.Error())
					}

					if p.MaxSolCost == nil {
						return errNoCreatorBuy
					}

					associatedUser := p.GetAssociatedUserAccount()
					if associatedUser == nil {
						return errNoCreatorATA
					}

					c.creatorPurchased = true
					c.creatorPurchaseSol = 0.99 * float64(*p.MaxSolCost) / float64(solana.LAMPORTS_PER_SOL)
					c.creatorATA = associatedUser.PublicKey
					return nil
				}
			}
		}
	}

	return errNoCreatorBuy
}

func (b *Bot) shouldBuyCoin(coin *Coin) bool {
	// check price constraints
	var creatorPubKey = coin.creator.String()
	if coin.creatorPurchaseSol < 0.5 || coin.creatorPurchaseSol > 2.5 {
		return false
	}

	// make sure creator's first coin
	if b.addressCreatedCoin(creatorPubKey) {
		return false
	}

	// check 30 past tx for all funders, not just first
	funderTrans, err := b.fetchNLastTrans(30, creatorPubKey)
	if err != nil {
		b.statusr("Error checking buy coin: " + err.Error())
		return false
	}

	// fetch up to 3 funders
	creatorFunders := findFundersFromResps(funderTrans, creatorPubKey, 3)
	if len(creatorFunders) == 0 {
		return false
	}

	var funderStatusChan = make(chan bool)
	var safeFundersCount int

	for _, funder := range creatorFunders {
		go b.isSafeFunder(funder, funderStatusChan)
	}

	for i := 0; i < len(creatorFunders); i++ {
		safe := <-funderStatusChan
		if safe {
			safeFundersCount++
		}
	}

	return safeFundersCount == len(creatorFunders)
}

func (b *Bot) isSafeFunder(funder string, funderStatusChan chan bool) {
	if isExchangeAddress(funder) {
		funderStatusChan <- true
		return
	}

	if b.addressCreatedCoin(funder) {
		funderStatusChan <- false
		return
	}

	// TODO: add back if we want to sacrifice speed (or can afford to)

	// // do second check against the funding wallets
	// // but only for the first funder found, as this covers most
	// // pump & dump creators

	// secondOrderFunderTrans, err := b.fetchNLastTrans(5, funder)
	// if err != nil {
	// 	b.statusr("Error Fetching 2nd Order Funder Trans: " + err.Error())
	// 	funderStatusChan <- false
	// 	return
	// }

	// secondOrderFunders := findFundersFromResps(secondOrderFunderTrans, funder, 1)

	// // if we can't find the second funder, assume they are good
	// if len(secondOrderFunders) == 0 {
	// 	funderStatusChan <- true
	// 	return
	// }

	// secondOrderFunder := secondOrderFunders[0]
	// if isExchangeAddress(secondOrderFunder) {
	// 	funderStatusChan <- true
	// 	return
	// }

	// if b.addressCreatedCoin(secondOrderFunder) {
	// 	funderStatusChan <- false
	// }
}

func (b *Bot) addressCreatedCoin(creatorAddress string) bool {
	query := "SELECT COUNT(*) FROM coins WHERE creator_address = ?"

	var count int
	err := b.dbConnection.QueryRow(query, creatorAddress).Scan(&count)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}

	return count > 0
}

func findFundersFromResps(responses jsonrpc.RPCResponses, creatorAddress string, fundersLimit int) []string {
	var funders []string

	for _, response := range responses {
		var transResult *rpc.GetTransactionResult = &rpc.GetTransactionResult{}
		if err := response.GetObject(&transResult); err != nil {
			continue
		}

		if transResult == nil || transResult.Transaction == nil {
			continue
		}

		tx, err := transResult.Transaction.GetTransaction()
		if err != nil {
			continue
		}

		if tx == nil {
			continue
		}

		funder := checkHasFunder(tx, creatorAddress)
		if funder != "" {
			funders = append(funders, funder)
		}

		if len(funders) == fundersLimit {
			return funders
		}
	}

	return funders
}

func checkHasFunder(tx *solana.Transaction, creatorAddr string) string {
	for _, iAtIndex := range tx.Message.Instructions {

		// Find the accounts of this instruction:
		accounts, err := iAtIndex.ResolveInstructionAccounts(&tx.Message)
		if err != nil {
			continue
		}

		inst, err := system.DecodeInstruction(accounts, iAtIndex.Data)
		if err != nil {
			continue
		}

		transfer, ok := inst.Impl.(*system.Transfer)
		if !ok {
			continue
		}

		if transfer.Lamports == nil {
			continue
		}

		rawLamports := float64(*transfer.Lamports)
		solAmount := rawLamports / float64(solana.LAMPORTS_PER_SOL)
		funderAddr := transfer.GetFundingAccount().PublicKey.String()

		// TODO: consider updating this to be coin buy amount
		if funderAddr != creatorAddr && solAmount > 0.05 {
			// fmt.Println("Funder of", funderAddr)
			return funderAddr
		}
	}

	return ""
}

func isMintLog(logEntry string) bool {
	return strings.Contains(logEntry, "InitializeMint2")
}
