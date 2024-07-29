package blockengine_client

import (
	"context"
	"crypto/tls"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/pkg"
	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewRelayer(ctx context.Context, grpcDialURL string, privateKey solana.PrivateKey, tlsConfig *tls.Config, opts ...grpc.DialOption) (*Relayer, error) {
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

	blockEngineRelayerClient := proto.NewBlockEngineRelayerClient(conn)
	authService := pkg.NewAuthenticationService(conn, privateKey)
	if err = authService.AuthenticateAndRefresh(proto.Role_RELAYER); err != nil {
		return nil, err
	}

	return &Relayer{
		GrpcConn: conn,
		Client:   blockEngineRelayerClient,
		Auth:     pkg.NewAuthenticationService(conn, privateKey),
		ErrChan:  chErr,
	}, nil
}

func NewValidator(ctx context.Context, grpcDialURL string, privateKey solana.PrivateKey, tlsConfig *tls.Config, opts ...grpc.DialOption) (*Validator, error) {
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

	blockEngineValidatorClient := proto.NewBlockEngineValidatorClient(conn)
	authService := pkg.NewAuthenticationService(conn, privateKey)
	if err = authService.AuthenticateAndRefresh(proto.Role_VALIDATOR); err != nil {
		return nil, err
	}

	return &Validator{
		GrpcConn: conn,
		Client:   blockEngineValidatorClient,
		Auth:     authService,
		ErrChan:  chErr,
	}, nil
}

func (c *Validator) SubscribePackets() (proto.BlockEngineValidator_SubscribePacketsClient, error) {
	return c.Client.SubscribePackets(c.Auth.GrpcCtx, &proto.SubscribePacketsRequest{})
}

// OnPacketSubscription is a wrapper of SubscribePackets.
func (c *Validator) OnPacketSubscription(ctx context.Context) (<-chan *proto.SubscribePacketsResponse, <-chan error, error) {
	sub, err := c.SubscribePackets()
	if err != nil {
		return nil, nil, err
	}

	chPackets := make(chan *proto.SubscribePacketsResponse)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- err
					continue
				}

				chPackets <- resp
			}
		}
	}()

	return chPackets, chErr, nil
}

func (c *Validator) SubscribeBundles() (proto.BlockEngineValidator_SubscribeBundlesClient, error) {
	return c.Client.SubscribeBundles(c.Auth.GrpcCtx, &proto.SubscribeBundlesRequest{})
}

// OnBundleSubscription is a wrapper of SubscribeBundles.
func (c *Validator) OnBundleSubscription(ctx context.Context) (<-chan []*proto.BundleUuid, <-chan error, error) {
	sub, err := c.SubscribeBundles()
	if err != nil {
		return nil, nil, err
	}

	chBundleUuid := make(chan []*proto.BundleUuid)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Auth.GrpcCtx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- err
					continue
				}

				chBundleUuid <- resp.Bundles
			}
		}
	}()

	return chBundleUuid, chErr, nil
}

func (c *Validator) GetBlockBuilderFeeInfo(opts ...grpc.CallOption) (*proto.BlockBuilderFeeInfoResponse, error) {
	return c.Client.GetBlockBuilderFeeInfo(c.Auth.GrpcCtx, &proto.BlockBuilderFeeInfoRequest{}, opts...)
}

func (c *Relayer) SubscribeAccountsOfInterest(opts ...grpc.CallOption) (proto.BlockEngineRelayer_SubscribeAccountsOfInterestClient, error) {
	return c.Client.SubscribeAccountsOfInterest(c.Auth.GrpcCtx, &proto.AccountsOfInterestRequest{}, opts...)
}

// OnSubscribeAccountsOfInterest is a wrapper of SubscribeAccountsOfInterest.
func (c *Relayer) OnSubscribeAccountsOfInterest(ctx context.Context) (<-chan *proto.AccountsOfInterestUpdate, <-chan error, error) {
	sub, err := c.SubscribeAccountsOfInterest()
	if err != nil {
		return nil, nil, err
	}

	chAccountOfInterest := make(chan *proto.AccountsOfInterestUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Auth.GrpcCtx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- err
					continue
				}

				chAccountOfInterest <- resp
			}
		}
	}()

	return chAccountOfInterest, chErr, nil
}

func (c *Relayer) SubscribeProgramsOfInterest(opts ...grpc.CallOption) (proto.BlockEngineRelayer_SubscribeProgramsOfInterestClient, error) {
	return c.Client.SubscribeProgramsOfInterest(c.Auth.GrpcCtx, &proto.ProgramsOfInterestRequest{}, opts...)
}

// OnSubscribeProgramsOfInterest is a wrapper of SubscribeProgramsOfInterest.
func (c *Relayer) OnSubscribeProgramsOfInterest(ctx context.Context) (<-chan *proto.ProgramsOfInterestUpdate, <-chan error, error) {
	sub, err := c.SubscribeProgramsOfInterest()
	if err != nil {
		return nil, nil, err
	}

	chProgramsOfInterest := make(chan *proto.ProgramsOfInterestUpdate)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				subInfo, err := sub.Recv()
				if err != nil {
					chErr <- err
					continue
				}

				chProgramsOfInterest <- subInfo
			}
		}
	}()

	return chProgramsOfInterest, chErr, nil
}

func (c *Relayer) StartExpiringPacketStream(opts ...grpc.CallOption) (proto.BlockEngineRelayer_StartExpiringPacketStreamClient, error) {
	return c.Client.StartExpiringPacketStream(c.Auth.GrpcCtx, opts...)
}

// OnStartExpiringPacketStream is a wrapper of StartExpiringPacketStream.
func (c *Relayer) OnStartExpiringPacketStream(ctx context.Context) (<-chan *proto.StartExpiringPacketStreamResponse, <-chan error, error) {
	sub, err := c.StartExpiringPacketStream()
	if err != nil {
		return nil, nil, err
	}

	chPacket := make(chan *proto.StartExpiringPacketStreamResponse)
	chErr := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Auth.GrpcCtx.Done():
				return
			default:
				resp, err := sub.Recv()
				if err != nil {
					chErr <- err
					continue
				}

				chPacket <- resp
			}
		}
	}()

	return chPacket, chErr, nil
}
