package searcher_client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/pkg"
	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// New creates a new Searcher Client instance.
func New(ctx context.Context, grpcDialURL string, jitoRpcClient, rpcClient *rpc.Client, privateKey solana.PrivateKey, tlsConfig *tls.Config, opts ...grpc.DialOption) (*Client, error) {
	if tlsConfig != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}

	chErr := make(chan error)
	conn, err := pkg.CreateAndObserveGRPCConn(ctx, chErr, grpcDialURL, opts...)
	if err != nil {
		return nil, err
	}

	searcherService := proto.NewSearcherServiceClient(conn)
	authService := pkg.NewAuthenticationService(conn, privateKey)
	if err = authService.AuthenticateAndRefresh(proto.Role_SEARCHER); err != nil {
		return nil, err
	}

	subBundleRes, err := searcherService.SubscribeBundleResults(authService.GrpcCtx, &proto.SubscribeBundleResultsRequest{})
	if err != nil {
		return nil, err
	}

	return &Client{
		GrpcConn:                 conn,
		RpcConn:                  rpcClient,
		JitoRpcConn:              jitoRpcClient,
		SearcherService:          searcherService,
		BundleStreamSubscription: subBundleRes,
		Auth:                     authService,
		ErrChan:                  chErr,
	}, nil
}

// NewMempoolStreamAccount creates a new mempool subscription on specific Solana accounts.
func (c *Client) NewMempoolStreamAccount(accounts, regions []string) (proto.SearcherService_SubscribeMempoolClient, error) {
	return c.SearcherService.SubscribeMempool(c.Auth.GrpcCtx, &proto.MempoolSubscription{
		Msg: &proto.MempoolSubscription_WlaV0Sub{
			WlaV0Sub: &proto.WriteLockedAccountSubscriptionV0{
				Accounts: accounts,
			},
		},
		Regions: regions,
	})
}

// NewMempoolStreamProgram creates a new mempool subscription on specific Solana programs.
func (c *Client) NewMempoolStreamProgram(programs, regions []string) (proto.SearcherService_SubscribeMempoolClient, error) {
	return c.SearcherService.SubscribeMempool(c.Auth.GrpcCtx, &proto.MempoolSubscription{
		Msg: &proto.MempoolSubscription_ProgramV0Sub{
			ProgramV0Sub: &proto.ProgramSubscriptionV0{
				Programs: programs,
			},
		},
		Regions: regions,
	})
}

// SubscribeAccountsMempoolTransactions subscribes to the mempool transactions of the provided accounts.
func (c *Client) SubscribeAccountsMempoolTransactions(ctx context.Context, accounts, regions []string) (<-chan *solana.Transaction, <-chan error, error) {
	sub, err := c.NewMempoolStreamAccount(accounts, regions)
	if err != nil {
		return nil, nil, err
	}

	chTx := make(chan *solana.Transaction)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Auth.GrpcCtx.Done():
				return
			default:
				var receipt *proto.PendingTxNotification
				receipt, err = sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("SubscribeAccountsMempoolTransactions: failed to receive mempool notification: %w", err)
					continue
				}

				for _, transaction := range receipt.Transactions {
					go func(transaction *proto.Packet) {
						var tx *solana.Transaction
						tx, err = pkg.ConvertProtobufPacketToTransaction(transaction)
						if err != nil {
							chErr <- fmt.Errorf("SubscribeAccountsMempoolTransactions: failed to convert protobuf packet to transaction: %w", err)
							return
						}

						chTx <- tx
					}(transaction)
				}
			}
		}
	}()

	return chTx, chErr, nil
}

// SubscribeProgramsMempoolTransactions subscribes to the mempool transactions of the provided programs.
func (c *Client) SubscribeProgramsMempoolTransactions(ctx context.Context, programs, regions []string) (<-chan *solana.Transaction, <-chan error, error) {
	sub, err := c.NewMempoolStreamProgram(programs, regions)
	if err != nil {
		return nil, nil, err
	}

	chTx := make(chan *solana.Transaction)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Auth.GrpcCtx.Done():
				return
			default:
				var receipt *proto.PendingTxNotification
				receipt, err = sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("SubscribeProgramsMempoolTransactions: failed to receive mempool notification: %w", err)
					continue
				}

				for _, transaction := range receipt.Transactions {
					go func(transaction *proto.Packet) {
						var tx *solana.Transaction
						tx, err = pkg.ConvertProtobufPacketToTransaction(transaction)
						if err != nil {
							chErr <- fmt.Errorf("SubscribeProgramsMempoolTransactions: failed to convert protobuf packet to transaction: %w", err)
							return
						}

						chTx <- tx
					}(transaction)
				}
			}
		}
	}()

	return chTx, chErr, nil
}

func (c *Client) GetRegions(opts ...grpc.CallOption) (*proto.GetRegionsResponse, error) {
	return c.SearcherService.GetRegions(c.Auth.GrpcCtx, &proto.GetRegionsRequest{}, opts...)
}

func (c *Client) GetConnectedLeaders(opts ...grpc.CallOption) (*proto.ConnectedLeadersResponse, error) {
	return c.SearcherService.GetConnectedLeaders(c.Auth.GrpcCtx, &proto.ConnectedLeadersRequest{}, opts...)
}

func (c *Client) GetConnectedLeadersRegioned(regions []string, opts ...grpc.CallOption) (*proto.ConnectedLeadersRegionedResponse, error) {
	return c.SearcherService.GetConnectedLeadersRegioned(c.Auth.GrpcCtx, &proto.ConnectedLeadersRegionedRequest{Regions: regions}, opts...)
}

func (c *Client) GetTipAccounts(opts ...grpc.CallOption) (*proto.GetTipAccountsResponse, error) {
	return c.SearcherService.GetTipAccounts(c.Auth.GrpcCtx, &proto.GetTipAccountsRequest{}, opts...)
}

// GetRandomTipAccount returns a random Jito TipAccount.
func (c *Client) GetRandomTipAccount(opts ...grpc.CallOption) (string, error) {
	resp, err := c.GetTipAccounts(opts...)
	if err != nil {
		return "", err
	}

	return resp.Accounts[rand.Intn(len(resp.Accounts))], nil
}

func (c *Client) GetNextScheduledLeader(regions []string, opts ...grpc.CallOption) (*proto.NextScheduledLeaderResponse, error) {
	return c.SearcherService.GetNextScheduledLeader(c.Auth.GrpcCtx, &proto.NextScheduledLeaderRequest{Regions: regions}, opts...)
}

// NewBundleSubscriptionResults creates a new bundle subscription, allowing to receive information about broadcasted bundles.
func (c *Client) NewBundleSubscriptionResults(opts ...grpc.CallOption) (proto.SearcherService_SubscribeBundleResultsClient, error) {
	return c.SearcherService.SubscribeBundleResults(c.Auth.GrpcCtx, &proto.SubscribeBundleResultsRequest{}, opts...)
}

// BroadcastBundle sends a bundle of transactions on chain thru Jito.
func (c *Client) BroadcastBundle(transactions []*solana.Transaction, opts ...grpc.CallOption) (*proto.SendBundleResponse, error) {
	bundle, err := c.AssembleBundle(transactions)
	if err != nil {
		return nil, err
	}

	return c.SearcherService.SendBundle(c.Auth.GrpcCtx, &proto.SendBundleRequest{Bundle: bundle}, opts...)
}

// BroadcastBundleWithConfirmation sends a bundle of transactions on chain thru Jito BlockEngine and waits for its confirmation.
func (c *Client) BroadcastBundleWithConfirmation(ctx context.Context, transactions []*solana.Transaction, opts ...grpc.CallOption) (*proto.SendBundleResponse, error) {
	bundleSignatures := pkg.BatchExtractSigFromTx(transactions)

	resp, err := c.BroadcastBundle(transactions, opts...)
	if err != nil {
		return nil, err
	}

	retries := 5
	for i := 0; i < retries; i++ {
		select {
		case <-c.Auth.GrpcCtx.Done():
			return nil, c.Auth.GrpcCtx.Err()
		default:

			// waiting 5s to check bundle result
			time.Sleep(5 * time.Second)

			var bundleResult *proto.BundleResult
			bundleResult, err = c.BundleStreamSubscription.Recv()
			if err != nil {
				continue
			}

			if err = c.handleBundleResult(bundleResult); err != nil {
				return nil, err
			}

			var start = time.Now()
			var statuses *rpc.GetSignatureStatusesResult

			for {
				statuses, err = c.RpcConn.GetSignatureStatuses(ctx, false, bundleSignatures...)
				if err != nil {
					return nil, err
				}
				ready := true

				for _, status := range statuses.Value {
					if status == nil {
						ready = false
						break
					}
				}

				if ready {
					break
				}

				if time.Since(start) > 15*time.Second {
					return nil, errors.New("operation timed out after 15 seconds")
				} else {
					time.Sleep(1 * time.Second)
				}
			}

			for _, status := range statuses.Value {
				if status.ConfirmationStatus != rpc.ConfirmationStatusProcessed && status.ConfirmationStatus != rpc.ConfirmationStatusConfirmed {
					return nil, errors.New("searcher service did not provide bundle status in time")
				}
			}

			return resp, nil
		}
	}

	return nil, fmt.Errorf("BroadcastBundleWithConfirmation error: max retries exceeded")
}

func (c *Client) handleBundleResult(bundleResult *proto.BundleResult) error {
	switch bundleResult.Result.(type) {
	case *proto.BundleResult_Accepted:
		break
	case *proto.BundleResult_Rejected:
		rejected := bundleResult.Result.(*proto.BundleResult_Rejected)
		switch rejected.Rejected.Reason.(type) {
		case *proto.Rejected_SimulationFailure:
			rejection := rejected.Rejected.GetSimulationFailure()
			return NewSimulationFailureError(rejection.TxSignature, rejection.GetMsg())
		case *proto.Rejected_StateAuctionBidRejected:
			rejection := rejected.Rejected.GetStateAuctionBidRejected()
			return NewStateAuctionBidRejectedError(rejection.AuctionId, rejection.SimulatedBidLamports)
		case *proto.Rejected_WinningBatchBidRejected:
			rejection := rejected.Rejected.GetWinningBatchBidRejected()
			return NewWinningBatchBidRejectedError(rejection.AuctionId, rejection.SimulatedBidLamports)
		case *proto.Rejected_InternalError:
			rejection := rejected.Rejected.GetInternalError()
			return NewInternalError(rejection.Msg)
		case *proto.Rejected_DroppedBundle:
			rejection := rejected.Rejected.GetDroppedBundle()
			return NewDroppedBundle(rejection.Msg)
		default:
			return nil
		}
	}
	return nil
}

// SimulateBundle is an RPC method that simulates a Jito bundle – exclusively available to Jito-Solana validator.
func (c *Client) SimulateBundle(ctx context.Context, bundleParams SimulateBundleParams, simulationConfigs SimulateBundleConfig) (*SimulatedBundleResponse, error) {
	out := new(SimulatedBundleResponse)

	if len(bundleParams.EncodedTransactions) != len(simulationConfigs.PreExecutionAccountsConfigs) {
		return nil, errors.New("pre/post execution account config length must match bundle length")
	}

	err := c.JitoRpcConn.RPCCallForInto(ctx, out, "simulateBundle", []interface{}{bundleParams, simulationConfigs})
	return out, err
}

func (c *Client) GetBundleStatuses(ctx context.Context, bundleIDs []string) (*BundleStatusesResponse, error) {
	if len(bundleIDs) > 5 {
		return nil, fmt.Errorf("max length reached (exp 5, got %d), please use BatchGetBundleStatuses  or reduce the amt of bundle", len(bundleIDs))
	}

	var params []interface{}
	for _, bundleID := range bundleIDs {
		params = append(params, bundleID)
	}

	out := new(BundleStatusesResponse)
	err := c.JitoRpcConn.RPCCallForInto(ctx, out, "getBundleStatuses", params)

	return out, err
}

func (c *Client) BatchGetBundleStatuses(ctx context.Context, bundleIDs ...string) ([]*BundleStatusesResponse, error) {
	if len(bundleIDs) > 5 {
		var bundles [][]string
		var out []*BundleStatusesResponse

		for _, bundleID := range bundleIDs {
			if len(bundles) == 0 || len(bundles[len(bundles)-1]) == 5 {
				bundles = append(bundles, []string{bundleID})
			} else {
				bundles[len(bundles)-1] = append(bundles[len(bundles)-1], bundleID)
			}
		}

		for _, bundle := range bundles {
			resp, err := c.GetBundleStatuses(ctx, bundle)
			if err != nil {
				return out, err
			}

			out = append(out, resp)
		}

		return out, nil
	} else {
		var out []*BundleStatusesResponse

		resp, err := c.GetBundleStatuses(ctx, bundleIDs)
		if err != nil {
			return nil, err
		}

		out = append(out, resp)

		return out, nil
	}
}

func GetBundleStatuses(client *http.Client, bundleIDs []string) (*BundleStatusesResponse, error) {
	if len(bundleIDs) > 5 {
		return nil, fmt.Errorf("max length reached (exp 5, got %d), please use BatchGetBundleStatuses or reduce the amt of bundle", len(bundleIDs))
	}

	buf := new(bytes.Buffer)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getBundleStatuses",
		"params": [][]string{
			bundleIDs,
		},
	}

	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodPost,
		URL:    jitoURL,
		Body:   io.NopCloser(buf),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing GetBundleStatuses: client error > %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	data := new(BundleStatusesResponse)
	err = json.Unmarshal(body, &data)

	return data, err
}

func BatchGetBundleStatuses(client *http.Client, bundleIDs ...string) ([]*BundleStatusesResponse, error) {
	if len(bundleIDs) > 5 {
		var bundles [][]string
		var out []*BundleStatusesResponse

		for _, bundleID := range bundleIDs {
			if len(bundles) == 0 || len(bundles[len(bundles)-1]) == 5 {
				bundles = append(bundles, []string{bundleID})
			} else {
				bundles[len(bundles)-1] = append(bundles[len(bundles)-1], bundleID)
			}
		}

		for _, bundle := range bundles {
			resp, err := GetBundleStatuses(client, bundle)
			if err != nil {
				return out, err
			}

			out = append(out, resp)
		}

		return out, nil
	} else {
		var out []*BundleStatusesResponse

		resp, err := GetBundleStatuses(client, bundleIDs)
		if err != nil {
			return nil, err
		}

		out = append(out, resp)

		return out, nil
	}
}

func (c *Client) AssembleBundle(transactions []*solana.Transaction) (*proto.Bundle, error) {
	packets := make([]*proto.Packet, 0, len(transactions))

	// converts an array of transactions to an array of protobuf packets
	for i, tx := range transactions {
		packet, err := pkg.ConvertTransactionToProtobufPacket(tx)
		if err != nil {
			return nil, fmt.Errorf("%d: error converting tx to proto packet [%w]", i, err)
		}

		packets = append(packets, &packet)
	}

	return &proto.Bundle{Packets: packets, Header: nil}, nil
}

// ValidateTransaction makes sure the bytes length of your transaction < 1232.
// If your transaction is bigger, Jito will return an error.
func ValidateTransaction(tx *solana.Transaction) bool {
	return len([]byte(tx.String())) <= 1232
}

// GenerateTipInstruction is a function that generates a Solana tip instruction mandatory to broadcast a bundle to Jito.
func GenerateTipInstruction(tipAmount uint64, from, tipAccount solana.PublicKey) solana.Instruction {
	return system.NewTransferInstruction(tipAmount, from, tipAccount).Build()
}

// GenerateTipRandomAccountInstruction functions similarly to GenerateTipInstruction, but it selects a random tip account.
func (c *Client) GenerateTipRandomAccountInstruction(tipAmount uint64, from solana.PublicKey) (solana.Instruction, error) {
	tipAccount, err := c.GetRandomTipAccount()
	if err != nil {
		return nil, err
	}

	return system.NewTransferInstruction(tipAmount, from, solana.MustPublicKeyFromBase58(tipAccount)).Build(), nil
}

type BundleRejectionError struct {
	Message string
}

func (e BundleRejectionError) Error() string {
	return e.Message
}

func NewStateAuctionBidRejectedError(auction string, tip uint64) error {
	return BundleRejectionError{
		Message: fmt.Sprintf("bundle lost state auction, auction: %s, tip %d lamports", auction, tip),
	}
}

func NewWinningBatchBidRejectedError(auction string, tip uint64) error {
	return BundleRejectionError{
		Message: fmt.Sprintf("bundle won state auction but failed global auction, auction %s, tip %d lamports", auction, tip),
	}
}

func NewSimulationFailureError(tx string, message string) error {
	return BundleRejectionError{
		Message: fmt.Sprintf("bundle simulation failure on tx %s, message: %s", tx, message),
	}
}

func NewInternalError(message string) error {
	return BundleRejectionError{
		Message: fmt.Sprintf("internal error %s", message),
	}
}

func NewDroppedBundle(message string) error {
	return BundleRejectionError{
		Message: fmt.Sprintf("bundle dropped %s", message),
	}
}
