package api

import (
	"errors"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/worker/protobuf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"

	"context"
	"fmt"
	"net"
	"time"
)

type FromWorkerServer struct {
	Dao dao.DaoWrapper

	protobuf.UnimplementedFromWorkerServer
}

func (f *FromWorkerServer) Register(ctx context.Context, req *protobuf.RegisterRequest) (*protobuf.RegisterResponse, error) {
	worker, err := f.Dao.GetWorkerByHostname(ctx, req.Hostname)
	if err != nil {
		worker.Host = req.Hostname
		err := f.Dao.WorkerDao.CreateWorker(&worker)
		if err != nil {
			return nil, err
		}
	}

	return &protobuf.RegisterResponse{
		Id: uint64(worker.ID),
	}, nil
}

func (f *FromWorkerServer) Heartbeat(ctx context.Context, req *protobuf.HeartbeatRequest) (*protobuf.HeartbeatResponse, error) {
	if worker, err := f.Dao.GetWorkerByID(ctx, uint(req.GetID())); err != nil {
		return nil, errors.New("unknown worker id")
	} else {
		worker.Workload = uint(req.Workload)
		worker.LastSeen = time.Now()
		worker.CPU = req.CPU
		worker.Memory = req.Memory
		worker.Disk = req.Disk
		worker.Uptime = req.Uptime
		worker.Version = req.Version
		err := f.Dao.SaveWorker(worker)
		if err != nil {
			return nil, err
		}
		return &protobuf.HeartbeatResponse{}, nil
	}
}

// InitApiGrpc Initializes api endpoints
// addr: port to run on, e.g. ":8080"
func (f *FromWorkerServer) InitApiGrpc(addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterFromWorkerServer(grpcServer, f)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func StartDueStreams(dao dao.DaoWrapper) func() {
	return func() {

	}
	// todo
}

func FetchLivePreviews(dao dao.DaoWrapper) func() {
	return func() {

	}
	// todo
}

func triggerStream(stream model.Stream, v string) {
	conn, err := dialIn(model.WorkerV2{
		Host: "localhost",
	})
	if err != nil {
		return
	}
	client := protobuf.NewToWorkerClient(conn)
	job, err := client.RequestStream(context.Background(), &protobuf.StreamRequest{
		Stream:  uint64(stream.ID),
		Course:  uint64(stream.CourseID),
		Version: v,
		End:     timestamppb.New(stream.End),
	})
	if err != nil {
		return
	}
	fmt.Println(job.GetJob())
}

func dialIn(targetWorker model.WorkerV2) (*grpc.ClientConn, error) {
	credentials := insecure.NewCredentials()
	log.Info("Connecting to:" + fmt.Sprintf("%s:50051", targetWorker.Host))
	conn, err := grpc.Dial(fmt.Sprintf("%s:50051", targetWorker.Host), grpc.WithTransportCredentials(credentials))
	return conn, err
}
