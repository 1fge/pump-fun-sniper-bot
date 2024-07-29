package geyser_client

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/pkg"
	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// New creates a new RPC client and connects to the provided endpoint. A Geyser RPC URL is required.
func New(ctx context.Context, grpcDialURL string, tlsConfig *tls.Config, opts ...grpc.DialOption) (*Client, error) {
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

	geyserClient := proto.NewGeyserClient(conn)

	return &Client{
		GrpcConn: conn,
		Ctx:      ctx,
		Geyser:   geyserClient,
		ErrChan:  chErr,
	}, nil
}

func (c *Client) SubscribePartialAccountUpdates(opts ...grpc.CallOption) (proto.Geyser_SubscribePartialAccountUpdatesClient, error) {
	return c.Geyser.SubscribePartialAccountUpdates(c.Ctx, &proto.SubscribePartialAccountUpdatesRequest{SkipVoteAccounts: true}, opts...)
}

// OnPartialAccountUpdates is a wrapper of SubscribePartialAccountUpdates.
func (c *Client) OnPartialAccountUpdates(ctx context.Context) (<-chan *proto.PartialAccountUpdate, <-chan error, error) {
	sub, err := c.SubscribePartialAccountUpdates()
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan *proto.PartialAccountUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				subInfo, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnPartialAccountUpdates: %w", err)
					continue
				}

				ch <- subInfo.GetPartialAccountUpdate()
			}
		}
	}()

	return ch, chErr, nil
}

func (c *Client) SubscribeBlockUpdates(opts ...grpc.CallOption) (proto.Geyser_SubscribeBlockUpdatesClient, error) {
	return c.Geyser.SubscribeBlockUpdates(c.Ctx, &proto.SubscribeBlockUpdatesRequest{}, opts...)
}

// OnBlockUpdates is a wrapper of SubscribeBlockUpdates.
func (c *Client) OnBlockUpdates(ctx context.Context) (<-chan *proto.TimestampedBlockUpdate, <-chan error, error) {
	sub, err := c.SubscribeBlockUpdates()
	if err != nil {
		return nil, nil, err
	}

	chBlock := make(chan *proto.TimestampedBlockUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnBlockUpdates: %w", err)
					continue
				}

				chBlock <- resp
			}
		}
	}()

	return chBlock, chErr, nil
}

func (c *Client) SubscribeAccountUpdates(accounts []string, opts ...grpc.CallOption) (proto.Geyser_SubscribeAccountUpdatesClient, error) {
	return c.Geyser.SubscribeAccountUpdates(c.Ctx, &proto.SubscribeAccountUpdatesRequest{Accounts: pkg.StrSliceToByteSlice(accounts)}, opts...)
}

// OnAccountUpdates is a wrapper of SubscribeAccountUpdates.
func (c *Client) OnAccountUpdates(ctx context.Context, accounts []string, opts ...grpc.CallOption) (<-chan *proto.TimestampedAccountUpdate, <-chan error, error) {
	sub, err := c.SubscribeAccountUpdates(accounts, opts...)
	if err != nil {
		return nil, nil, err
	}

	chAccount := make(chan *proto.TimestampedAccountUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnAccountUpdates: %w", err)
					continue
				}

				chAccount <- resp
			}
		}
	}()

	return chAccount, chErr, nil
}

func (c *Client) SubscribeProgramUpdates(programs []string, opts ...grpc.CallOption) (proto.Geyser_SubscribeProgramUpdatesClient, error) {
	return c.Geyser.SubscribeProgramUpdates(c.Ctx, &proto.SubscribeProgramsUpdatesRequest{Programs: pkg.StrSliceToByteSlice(programs)}, opts...)
}

// OnProgramUpdates is a wrapper of SubscribeProgramUpdates.
func (c *Client) OnProgramUpdates(ctx context.Context, programs []string, opts ...grpc.CallOption) (<-chan *proto.TimestampedAccountUpdate, <-chan error, error) {
	sub, err := c.SubscribeProgramUpdates(programs, opts...)
	if err != nil {
		return nil, nil, err
	}

	chProgram := make(chan *proto.TimestampedAccountUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnProgramUpdate: %w", err)
					continue
				}

				chProgram <- resp
			}
		}
	}()

	return chProgram, chErr, nil
}

func (c *Client) SubscribeTransactionUpdates(opts ...grpc.CallOption) (proto.Geyser_SubscribeTransactionUpdatesClient, error) {
	return c.Geyser.SubscribeTransactionUpdates(c.Ctx, &proto.SubscribeTransactionUpdatesRequest{}, opts...)
}

// OnTransactionUpdates is a wrapper of SubscribeTransactionUpdates.
func (c *Client) OnTransactionUpdates(ctx context.Context) (<-chan *proto.TimestampedTransactionUpdate, <-chan error, error) {
	sub, err := c.SubscribeTransactionUpdates()
	if err != nil {
		return nil, nil, err
	}

	chTx := make(chan *proto.TimestampedTransactionUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnTransactionUpdates: %w", err)
					continue
				}

				chTx <- resp
			}
		}
	}()

	return chTx, chErr, err
}

func (c *Client) SubscribeSlotUpdates(opts ...grpc.CallOption) (proto.Geyser_SubscribeSlotUpdatesClient, error) {
	return c.Geyser.SubscribeSlotUpdates(c.Ctx, &proto.SubscribeSlotUpdateRequest{}, opts...)
}

// OnSlotUpdates is a wrapper of SubscribeSlotUpdates.
func (c *Client) OnSlotUpdates(ctx context.Context, opts ...grpc.CallOption) (<-chan *proto.TimestampedSlotUpdate, <-chan error, error) {
	sub, err := c.SubscribeSlotUpdates(opts...)
	if err != nil {
		return nil, nil, err
	}

	chSlot := make(chan *proto.TimestampedSlotUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- fmt.Errorf("error OnSlotUpdates: %w", err)
					continue
				}

				chSlot <- resp
			}
		}
	}()

	return chSlot, chErr, nil
}
