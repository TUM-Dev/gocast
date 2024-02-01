package server

import (
	"fmt"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/logging"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"log/slog"
	"net"
	"os"
	"time"
)

type Server struct {
	cfg *config.EnvConfig
	log *slog.Logger

	GRPCServer *grpc.Server

	protobuf.UnimplementedToRunnerServer
}

var Instance *Server

func InitServer(cfg *config.EnvConfig, log *slog.Logger) {
	Instance = &Server{
		cfg: cfg,
		log: log,
	}
	Instance.Initialize()
}

func (s *Server) Initialize() {
	s.log.Info("Starting gRPC server", "port", s.cfg.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		s.log.Error("failed to listen", "error", err)
		os.Exit(1)
	}
	s.GRPCServer = grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}), logging.GetGrpcLogInterceptor(s.log))
	protobuf.RegisterToRunnerServer(s.GRPCServer, s)

	reflection.Register(s.GRPCServer)
	if err := s.GRPCServer.Serve(lis); err != nil {
		s.log.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}

// dialIn connects to manager instance and returns a client
func (s *Server) DialIn() (protobuf.FromRunnerClient, error) {
	credentials := insecure.NewCredentials()
	conn, err := grpc.Dial(s.cfg.GocastServer, grpc.WithTransportCredentials(credentials))
	if err != nil {
		return nil, err
	}
	return protobuf.NewFromRunnerClient(conn), nil
}
