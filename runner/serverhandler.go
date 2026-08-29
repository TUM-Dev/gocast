package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const registerRetries = 5

func (r *Runner) RegisterWithGocast(retries int) {
	r.log.Debug("connecting with gocast", slog.Group("conn", "host", config.Config.GocastServer, "retriesLeft", retries))
	if retries == 0 {
		r.log.Error("no more retries left, can't connect to gocast")
		os.Exit(1)
	}
	con, err := r.getManagerClient()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err, "sleeping(s)", registerRetries-retries)
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		return
	}
	_, err = con.Register(context.Background(), &protobuf.RegisterRequest{Hostname: ptr.Take(config.Config.Hostname), Port: ptr.Take(int32(config.Config.Port)), Version: ptr.Take(r.Version)})
	if err != nil {
		r.log.Error("error registering with gocast", "error", err, "sleeping(s)", registerRetries-retries)
		r.invalidateManagerConn()
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		return
	}
}

// getManagerClient returns a cached gRPC client to the manager, creating a new connection if needed.
func (r *Runner) getManagerClient() (protobuf.RunnerManagerServiceClient, error) {
	r.connMu.Lock()
	defer r.connMu.Unlock()

	if r.managerClient != nil && r.managerConn != nil {
		return r.managerClient, nil
	}

	conn, err := grpc.NewClient(config.Config.GocastServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("getManagerClient: %w", err)
	}
	r.managerConn = conn
	r.managerClient = protobuf.NewRunnerManagerServiceClient(conn)
	return r.managerClient, nil
}

// invalidateManagerConn closes the cached connection so the next call to getManagerClient creates a new one.
func (r *Runner) invalidateManagerConn() {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	if r.managerConn != nil {
		_ = r.managerConn.Close()
		r.managerConn = nil
		r.managerClient = nil
	}
}
