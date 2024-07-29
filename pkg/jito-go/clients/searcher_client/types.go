package searcher_client

import (
	"math/big"
	"net/url"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/pkg"
	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"github.com/gagliardetto/solana-go/rpc"
	"google.golang.org/grpc"
)

var jitoURL = &url.URL{
	Scheme: "https",
	Host:   "mainnet.block-engine.jito.wtf",
	Path:   "/api/v1/bundles",
}

type Client struct {
	GrpcConn    *grpc.ClientConn
	RpcConn     *rpc.Client // Utilized for executing standard Solana RPC requests.
	JitoRpcConn *rpc.Client // Utilized for executing specific Jito RPC requests (Jito node required).

	SearcherService          proto.SearcherServiceClient
	BundleStreamSubscription proto.SearcherService_SubscribeBundleResultsClient // Used for receiving *proto.BundleResult (bundle broadcast status info).

	Auth *pkg.AuthenticationService

	ErrChan <-chan error // ErrChan is used for dispatching errors from functions executed within goroutines.
}

type SimulateBundleConfig struct {
	PreExecutionAccountsConfigs  []ExecutionAccounts `json:"preExecutionAccountsConfigs"`
	PostExecutionAccountsConfigs []ExecutionAccounts `json:"postExecutionAccountsConfigs"`
}

type ExecutionAccounts struct {
	Encoding  string   `json:"encoding"`
	Addresses []string `json:"addresses"`
}

type SimulateBundleParams struct {
	EncodedTransactions []string `json:"encodedTransactions"`
}

type SimulatedBundleResponse struct {
	Context interface{}                   `json:"context"`
	Value   SimulatedBundleResponseStruct `json:"value"`
}

type SimulatedBundleResponseStruct struct {
	Summary           interface{}         `json:"summary"`
	TransactionResult []TransactionResult `json:"transactionResults"`
}

type TransactionResult struct {
	Err                   interface{} `json:"err,omitempty"`
	Logs                  []string    `json:"logs,omitempty"`
	PreExecutionAccounts  []Account   `json:"preExecutionAccounts,omitempty"`
	PostExecutionAccounts []Account   `json:"postExecutionAccounts,omitempty"`
	UnitsConsumed         *int        `json:"unitsConsumed,omitempty"`
	ReturnData            *ReturnData `json:"returnData,omitempty"`
}

type Account struct {
	Executable bool     `json:"executable"`
	Owner      string   `json:"owner"`
	Lamports   int      `json:"lamports"`
	Data       []string `json:"data"`
	RentEpoch  *big.Int `json:"rentEpoch,omitempty"`
}

type ReturnData struct {
	ProgramId string    `json:"programId"`
	Data      [2]string `json:"data"`
}

type BundleStatusesResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Context struct {
			Slot int `json:"slot"`
		} `json:"context"`
		Value []struct {
			BundleId           string   `json:"bundle_id"`
			Transactions       []string `json:"transactions"`
			Slot               int      `json:"slot"`
			ConfirmationStatus string   `json:"confirmation_status"`
			Err                struct {
				Ok interface{} `json:"Ok"`
			} `json:"err"`
		} `json:"value"`
	} `json:"result"`
	Id int `json:"id"`
}
