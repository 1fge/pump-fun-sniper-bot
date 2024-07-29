package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	jito_go "github.com/1fge/pump-fun-sniper-bot/pkg/jito-go"
	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/clients/searcher_client"
	util "github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type validatorAPIResponse struct {
	Validators []*jitoValidator `json:"validators"`
}

type jitoValidator struct {
	VoteAccount string `json:"vote_account"`
	RunningJito bool   `json:"running_jito"`
}

// JitoManager acts as the struct where we store important information for interacting
// with Jito. This includes keeping track of the leaders, determining tip amount based on percentile,
// and building the tip instruction.
type JitoManager struct {
	client    *http.Client
	rpcClient *rpc.Client

	privateKey solana.PrivateKey

	slotIndex uint64
	epoch     uint64

	// jitoValidators is a map of validator IDs that are running Jito.
	jitoValidators map[string]bool

	// slotLeader maps slot to validator ID.
	slotLeader map[uint64]string

	// voteAccounts maps nodeAccount to voteAccount
	voteAccounts map[string]string

	lock *sync.Mutex

	// tipInfo maps the latest tip information from Jito.
	tipInfo    *util.TipStreamInfo
	jitoClient *searcher_client.Client
}

func newJitoManager(rpcClient *rpc.Client, privateKey solana.PrivateKey) (*JitoManager, error) {
	jitoClient, err := searcher_client.New(
		context.Background(),
		jito_go.NewYork.BlockEngineURL,
		rpcClient,
		rpcClient,
		privateKey,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &JitoManager{
		client:     &http.Client{},
		rpcClient:  rpcClient,
		jitoClient: jitoClient,

		jitoValidators: make(map[string]bool),
		slotLeader:     make(map[uint64]string),
		voteAccounts:   make(map[string]string),

		lock: &sync.Mutex{},

		privateKey: privateKey,
	}, nil
}

func (j *JitoManager) status(msg string) {
	log.Println("Jito Manager", msg)
}

func (j *JitoManager) statusr(msg string) {
	log.Println("Jito Manager (R)", msg)
}

func (j *JitoManager) generateTipInstruction() (solana.Instruction, error) {
	tipAmount := j.generateTipAmount()
	j.status(fmt.Sprintf("Generating tip instruction for %.5f SOL", float64(tipAmount)/1e9))
	return j.jitoClient.GenerateTipRandomAccountInstruction(tipAmount, j.privateKey.PublicKey())
}

func (j *JitoManager) generateTipAmount() uint64 {
	if j.tipInfo == nil {
		return 2000000
	}

	return uint64(j.tipInfo.LandedTips75ThPercentile * 1e9)
}

func (j *JitoManager) manageTipStream() {
	go func() {
		for {
			if err := j.subscribeTipStream(); err != nil {
				j.statusr("Error reading tip stream: " + err.Error())
			}
		}
	}()
}

func (j *JitoManager) subscribeTipStream() error {
	infoChan, errChan, err := util.SubscribeTipStream(context.TODO())
	if err != nil {
		return err
	}

	for {
		select {
		case info := <-infoChan:
			j.status(fmt.Sprintf("Received tip stream (75th percentile=%.3fSOL, 95th percentile=%.3fSOL, 99th percentile=%.3fSOL)", info.LandedTips75ThPercentile, info.LandedTips95ThPercentile, info.LandedTips99ThPercentile))
			j.tipInfo = info
		case err = <-errChan:
			return err
		}
	}
}

func (j *JitoManager) start() error {
	if j.jitoClient == nil {
		return nil
	}

	j.manageTipStream()

	if err := j.fetchJitoValidators(); err != nil {
		return err
	}

	if err := j.fetchLeaderSchedule(); err != nil {
		return err
	}

	if err := j.fetchVoteAccounts(); err != nil {
		return err
	}

	if err := j.fetchEpochInfo(); err != nil {
		return err
	}

	go func() {
		for {
			if err := j.fetchEpochInfo(); err != nil {
				fmt.Println("Failed to fetch epoch info: ", err)
			}

			time.Sleep(10 * time.Millisecond)
		}
	}()

	go func() {
		for {
			if err := j.fetchLeaderSchedule(); err != nil {
				fmt.Println("Failed to fetch epoch info: ", err)
			}

			time.Sleep(10 * time.Minute)
		}
	}()

	go func() {
		for {
			if err := j.fetchJitoValidators(); err != nil {
				fmt.Println("Failed to fetch epoch info: ", err)
			}

			time.Sleep(10 * time.Minute)
		}
	}()

	go func() {
		for {
			if err := j.fetchVoteAccounts(); err != nil {
				fmt.Println("Failed to fetch epoch info: ", err)
			}

			time.Sleep(10 * time.Minute)
		}
	}()

	return nil
}

func (j *JitoManager) isJitoLeader() bool {
	j.lock.Lock()
	defer j.lock.Unlock()

	validator, ok := j.slotLeader[j.slotIndex]
	if !ok {
		return false
	}

	j.status("Checking if validator is a Jito leader: " + validator)
	isLeader := j.jitoValidators[j.voteAccounts[validator]]

	return isLeader
}

func (j *JitoManager) fetchLeaderSchedule() error {
	j.status("Fetching leader schedule")

	scheduleResult, err := j.rpcClient.GetLeaderSchedule(context.Background())
	if err != nil {
		return err
	}

	j.buildLeaderSchedule(&scheduleResult)

	return nil
}

func (j *JitoManager) buildLeaderSchedule(scheduleResult *rpc.GetLeaderScheduleResult) {
	j.lock.Lock()
	defer j.lock.Unlock()

	j.slotLeader = make(map[uint64]string)
	for validator, slots := range *scheduleResult {
		for _, slot := range slots {
			j.slotLeader[slot] = validator.String()
		}
	}
}

func (j *JitoManager) fetchVoteAccounts() error {
	j.status("Fetching vote accounts")

	voteAccounts, err := j.rpcClient.GetVoteAccounts(context.Background(), nil)
	if err != nil {
		return err
	}

	j.buildVoteAccounts(voteAccounts.Current)

	return nil
}

func (j *JitoManager) buildVoteAccounts(voteAccounts []rpc.VoteAccountsResult) {
	j.lock.Lock()
	defer j.lock.Unlock()

	for _, account := range voteAccounts {
		j.voteAccounts[account.NodePubkey.String()] = account.VotePubkey.String()
	}
}

func (j *JitoManager) fetchEpochInfo() error {
	schedule, err := j.rpcClient.GetEpochInfo(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}

	j.slotIndex = schedule.SlotIndex
	if j.epoch != schedule.Epoch {
		if err = j.fetchLeaderSchedule(); err != nil {
			return err
		}

		j.epoch = schedule.Epoch
	}

	return nil
}

// fetchJitoValidators fetches the list of validators from the Jito network.
func (j *JitoManager) fetchJitoValidators() error {
	j.status("Fetching jito-enabled validators")

	req, err := http.NewRequest("GET", "https://kobe.mainnet.jito.network/api/v1/validators", nil)
	if err != nil {
		return err
	}

	req.Header.Set("accept", "application/json")

	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to fetch validators: %s", resp.Status)
	}

	var validators validatorAPIResponse
	err = json.NewDecoder(resp.Body).Decode(&validators)
	if err != nil {
		return err
	}

	j.buildJitoValidators(validators.Validators)

	return nil
}

func (j *JitoManager) buildJitoValidators(validators []*jitoValidator) {
	j.lock.Lock()
	defer j.lock.Unlock()
	j.jitoValidators = make(map[string]bool)

	for i := range validators {
		if validators[i].RunningJito {
			j.jitoValidators[validators[i].VoteAccount] = true
		}
	}
}
