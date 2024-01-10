package api

import (
	"context"
	"database/sql"
	"errors"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	}

	r, err := g.RunnerDao.Get(ctx, runner.Hostname)
	if err != nil {
		log.WithError(err).Error("Failed to get runner")
		return &protobuf.HeartbeatResponse{Ok: false}, err
	}

	newStats := model.Runner{
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
	ctx = context.WithValue(ctx, "newStats", newStats)
	log.Info("Updating runner stats ", "runner", r)
	p, err := r.UpdateStats(dao.DB, ctx)
	return &protobuf.HeartbeatResponse{Ok: p}, err
}

// RequestSelfStream is called by the runner when a stream is supposed to be started by obs or other third party software
// returns an error if anything goes wrong OR the stream may not be published
func (g GrpcRunnerServer) RequestSelfStream(ctx context.Context, request *protobuf.SelfStreamRequest) (*protobuf.SelfStreamResponse, error) {
	//TODO Test me/Improve me
	if request.StreamKey == "" {
		return nil, errors.New("stream key is empty")
	}
	stream, err := g.StreamsDao.GetStreamByKey(ctx, request.StreamKey)
	if err != nil {
		return nil, err
	}
	course, err := g.CoursesDao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, err
	}
	if !(time.Now().After(stream.Start.Add(time.Minute*-30)) && time.Now().Before(stream.End.Add(time.Minute*30))) {
		log.WithFields(log.Fields{"streamId": stream.ID}).Warn("Stream rejected, time out of bounds")
		return nil, errors.New("stream rejected")
	}
	ingestServer, err := g.IngestServerDao.GetBestIngestServer()
	if err != nil {
		return nil, err
	}
	slot, err := g.IngestServerDao.GetStreamSlot(ingestServer.ID)
	if err != nil {
		return nil, err
	}
	slot.StreamID = stream.ID
	g.IngestServerDao.SaveSlot(slot)

	return &protobuf.SelfStreamResponse{
		Stream:       uint64(stream.ID),
		Course:       uint64(course.ID),
		CourseYear:   uint64(course.Year),
		StreamStart:  timestamppb.New(stream.Start),
		StreamEnd:    timestamppb.New(stream.End),
		UploadVoD:    course.VODEnabled,
		IngestServer: ingestServer.Url,
		StreamName:   stream.StreamName,
		OutURL:       ingestServer.OutUrl,
	}, nil
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
