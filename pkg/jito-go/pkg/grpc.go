package pkg

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"time"
)

// CreateAndObserveGRPCConn creates a new gRPC connection and observes its conn status.
func CreateAndObserveGRPCConn(ctx context.Context, chErr chan error, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}

	go func() {
		var retries int
		for {
			select {
			case <-ctx.Done():
				if err = conn.Close(); err != nil {
					chErr <- err
				}
				return
			default:
				state := conn.GetState()
				if state == connectivity.Ready {
					retries = 0
					time.Sleep(1 * time.Second)
					continue
				}

				if state == connectivity.TransientFailure || state == connectivity.Connecting || state == connectivity.Idle {
					if retries < 5 {
						time.Sleep(time.Duration(retries) * time.Second)
						conn.ResetConnectBackoff()
						retries++
					} else {
						conn.Close()
						conn, err = grpc.NewClient(target, opts...)
						if err != nil {
							chErr <- err
						}
						retries = 0
					}
				} else if state == connectivity.Shutdown {
					conn, err = grpc.NewClient(target, opts...)
					if err != nil {
						chErr <- err
					}
					retries = 0
				}

				if !conn.WaitForStateChange(ctx, state) {
					continue
				}
			}
		}
	}()

	return conn, nil
}
