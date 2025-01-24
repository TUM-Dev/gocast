package runner

import (
	"context"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"os"
	"time"
)

const registerRetries = 5

func (r *Runner) RegisterWithGocast(retries int) {
	r.log.Debug("connecting with gocast", slog.Group("conn", "host", config.Config.GocastServer, "retriesLeft", retries))
	if retries == 0 {
		r.log.Error("no more retries left, can't connect to gocast")
		os.Exit(1)
	}
	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err, "sleeping(s)", registerRetries-retries)
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		return
	}
	_, err = con.Register(context.Background(), &protobuf.RegisterRequest{Hostname: ptr.Take(config.Config.Hostname), Port: ptr.Take(int32(config.Config.Port))})
	if err != nil {
		r.log.Warn("error registering with gocast", "error", err, "sleeping(s)", registerRetries-retries)
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		return
	}
}

// dialIn connects to manager instance and returns a client
func (r *Runner) dialIn() (protobuf.FromRunnerClient, error) {
	credentials := insecure.NewCredentials()
	conn, err := grpc.Dial(config.Config.GocastServer, grpc.WithTransportCredentials(credentials))
	if err != nil {
		return nil, err
	}
	return protobuf.NewFromRunnerClient(conn), nil
}
