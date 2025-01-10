// voice_service_grpc.go handles communication between tum-live and voice-service

package api

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/voice-service/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

type subtitleReceiverServer struct {
	pb.UnimplementedSubtitleReceiverServer
	dao.DaoWrapper
}

func (s subtitleReceiverServer) Receive(_ context.Context, request *pb.ReceiveRequest) (*emptypb.Empty, error) {
	subtitlesEntry := model.Subtitles{
		StreamID: uint(request.GetStreamId()),
		Content:  request.GetSubtitles(),
		Language: request.GetLanguage(),
	}
	err := s.SubtitlesDao.CreateOrUpsert(context.Background(), &subtitlesEntry)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func init() {
	logger.Info("starting grpc voice-receiver")
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		logger.Error("failed to init voice-receiver server", "err", err)
		return
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	pb.RegisterSubtitleReceiverServer(grpcServer, &subtitleReceiverServer{DaoWrapper: dao.NewDaoWrapper()})

	reflection.Register(grpcServer)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("failed to serve", "err", err)
		}
	}()
}

type SubtitleGeneratorClient struct {
	pb.SubtitleGeneratorClient
	*grpc.ClientConn
}

func GetSubtitleGeneratorClient() (SubtitleGeneratorClient, error) {
	voiceAddr := fmt.Sprintf("%s:%s", tools.Cfg.VoiceService.Host, tools.Cfg.VoiceService.Port)
	conn, err := grpc.Dial(voiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return SubtitleGeneratorClient{}, err
	}
	return SubtitleGeneratorClient{pb.NewSubtitleGeneratorClient(conn), conn}, nil
}

func (s SubtitleGeneratorClient) CloseConn() {
	err := s.ClientConn.Close()
	if err != nil {
		logger.Error("could not close voice-service connection", "err", err)
	}
}
