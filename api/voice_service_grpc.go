package api

// voice_service_grpc.go handles communication between tum-live and voice-service
import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

type subtitleReceiverServer struct {
	pb.UnimplementedSubtitleReceiverServer
}

func (s subtitleReceiverServer) Receive(ctx context.Context, request *pb.ReceiveRequest) (*pb.Empty, error) {
	fmt.Println(request.GetSubtitles())
	return &pb.Empty{}, nil
}

func init() {
	log.Info("starting grpc voice-receiver")
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.WithError(err).Error("failed to init voice-receiver server")
		return
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	pb.RegisterSubtitleReceiverServer(grpcServer, &subtitleReceiverServer{})

	reflection.Register(grpcServer)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
		log.Info("dead")
	}()
}
