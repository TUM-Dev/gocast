package api

import (
	"context"
	"database/sql"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

var _ protobuf.FromRunnerServer = (*GrpcRunnerServer)(nil)

type GrpcRunnerServer struct {
	protobuf.UnimplementedFromRunnerServer

	dao.DaoWrapper
}

func (g GrpcRunnerServer) Register(ctx context.Context, request *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	runner := model.Runner{
		Hostname: request.Hostname,
		Port:     int(request.Port),
		LastSeen: sql.NullTime{Valid: true, Time: time.Now()},
		Status:   "Alive",
		Workload: 0,
	}
	err := g.RunnerDao.Create(ctx, &runner)
	if err != nil {
		return nil, err
	}
	return &protobuf.RegisterResponse{}, nil
}

func (g GrpcRunnerServer) Heartbeat(ctx context.Context, request *protobuf.HeartbeatRequest) (*protobuf.HeartbeatResponse, error) {
	runner := model.Runner{
		Hostname: request.Hostname,
		Port:     int(request.Port),
		LastSeen: sql.NullTime{Valid: true, Time: time.Now()},
		Status:   "Alive",
		Workload: uint(request.Workload),
		CPU:      request.CPU,
		Memory:   request.Memory,
		Disk:     request.Disk,
		Uptime:   request.Uptime,
		Version:  request.Version,
	}

	r, err := g.RunnerDao.Get(ctx, runner.Hostname)
	if err != nil {
		log.WithError(err).Error("Failed to get runner")
		return &protobuf.HeartbeatResponse{Ok: false}, err
	}
	ctx = context.WithValue(ctx, "runner", runner)
	p, err := r.UpdateStats(dao.DB, ctx)
	return &protobuf.HeartbeatResponse{Ok: p}, err
}

func (g GrpcRunnerServer) RequestSelfStream(ctx context.Context, request *protobuf.SelfStreamRequest) (*protobuf.SelfStreamResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (g GrpcRunnerServer) mustEmbedUnimplementedFromRunnerServer() {
	//TODO implement me
	panic("implement me")
}

func StartGrpcRunnerServer() {
	lis, err := net.Listen("tcp", ":50056")
	if err != nil {
		log.WithError(err).Error("Failed to init grpc server")
		return
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute * 5,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterFromRunnerServer(grpcServer, &GrpcRunnerServer{DaoWrapper: dao.NewDaoWrapper()})
	reflection.Register(grpcServer)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.WithError(err).Errorf("Can't serve grpc")
		}
	}()
}
