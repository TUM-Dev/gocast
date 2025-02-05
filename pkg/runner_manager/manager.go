package runner_manager

import (
	"context"
	"fmt"
	log "log/slog"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// Manager manages communication with runners and handles job distribution
type Manager struct {
	dao        dao.DaoWrapper
	listenAddr string

	protobuf.UnimplementedRunnerManagerServiceServer
}

// New returns a new instance of Manager with the given Options
func New(dao dao.DaoWrapper, opts ...Option) *Manager {
	m := Manager{dao: dao, listenAddr: ":50056"}
	m.applyOpts(opts)
	return &m
}

// Option is a func that applies configuration to the Manager
type Option func(m *Manager)

// WithListenAddr sets the address the Manager listens on for gRPC connections from the Runner.
// If not applied, the default is used (:50056)
func WithListenAddr(addr string) Option {
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	return func(m *Manager) {
		m.listenAddr = addr
	}
}

func (m *Manager) applyOpts(opts []Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *Manager) Run() error {
	lis, err := net.Listen("tcp", m.listenAddr)
	if err != nil {
		return fmt.Errorf("run manager: %v", err)
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterRunnerManagerServiceServer(grpcServer, m)
	reflection.Register(grpcServer)
	go func(listener net.Listener) {
		defer listener.Close()
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve runner manager", "err", err)
		}
	}(lis)
	return nil
}

func (m *Manager) Register(ctx context.Context, req *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	log.Info("Register Runner", "d", req)
	err := m.dao.RunnerDao.Create(ctx, &model.Runner{
		Hostname: req.GetHostname(),
		Port:     uint32(req.GetPort()),
	})
	if err != nil {
		return nil, fmt.Errorf("create runner: %v", err)
	}
	return &protobuf.RegisterResponse{}, nil
}

func (m *Manager) Notify(ctx context.Context, notification *protobuf.Notification) (*protobuf.NotificationResponse, error) {
	switch notification.Data.(type) {
	case *protobuf.Notification_Heartbeat:
		log.Debug("Heartbeat", "d", notification)
		runner, err := m.dao.RunnerDao.Get(ctx, notification.GetHeartbeat().GetHostname())
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "runner not found: %v", err)
		}
		runner.LastSeen = time.Now()
		runner.Draining = notification.GetHeartbeat().GetDraining()
		runner.JobCount = notification.GetHeartbeat().GetJobCount()
		err = m.dao.RunnerDao.Update(ctx, &runner)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "update runner: %v", err)
		}
		return &protobuf.NotificationResponse{}, nil
	default:
		return nil, status.Error(codes.Unimplemented, "unsupported notification type")
	}
}
