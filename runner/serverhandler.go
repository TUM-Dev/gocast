package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

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
		if shouldReconnect(err) {
			r.invalidateManagerConn(con)
		}
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

// managerConnGrace is how long a replaced manager connection stays open for the calls
// that are still running on it.
const managerConnGrace = time.Minute

// shouldReconnect tells whether err says the connection itself is broken. Everything else
// (a cancelled context, a deadline, the server's answer) says nothing about it: gRPC redials
// a lost connection on its own, and tearing a healthy one down aborts every call that is
// running on it.
func shouldReconnect(err error) bool {
	return status.Code(err) == codes.Unavailable
}

// invalidateManagerConn replaces the cached connection after a call on failed found it
// broken, so that the next getManagerClient dials a new one.
//
// The runner has many notifications in flight on this one connection. Closing it for every
// failed call made each failure abort all the others, which then failed and closed it
// again, so with enough of them waiting nothing was ever delivered. Hence: only the first
// failure on a connection replaces it (the others find it already gone), and the old
// connection is closed only after a grace period so that calls on it can finish.
func (r *Runner) invalidateManagerConn(failed protobuf.RunnerManagerServiceClient) {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	if r.managerConn == nil || r.managerClient != failed {
		return
	}
	old := r.managerConn
	r.managerConn = nil
	r.managerClient = nil
	time.AfterFunc(managerConnGrace, func() { _ = old.Close() })
}
