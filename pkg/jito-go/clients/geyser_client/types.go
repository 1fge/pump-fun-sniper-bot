package geyser_client

import (
	"context"

	"github.com/1fge/pump-fun-sniper-bot/pkg/jito-go/proto"
	"google.golang.org/grpc"
)

type Client struct {
	GrpcConn *grpc.ClientConn
	Ctx      context.Context

	Geyser proto.GeyserClient

	ErrChan <-chan error
}
