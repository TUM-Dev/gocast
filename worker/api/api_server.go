package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/joschahenningsen/thumbgen"

	"github.com/u2takey/go-utils/uuid"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/TUM-Dev/gocast/worker/cfg"
	"github.com/TUM-Dev/gocast/worker/pb"
	"github.com/TUM-Dev/gocast/worker/safepath"
	"github.com/TUM-Dev/gocast/worker/worker"
)

type server struct {
	pb.UnimplementedToWorkerServer
}

func (s server) RequestStream(ctx context.Context, request *pb.StreamRequest) (*pb.Status, error) {
	if request.WorkerId != cfg.WorkerID {
		log.Info("Rejected request to stream")
		return &pb.Status{Ok: false}, errors.New("unauthenticated: wrong worker id")
	}
	go worker.HandleStreamRequest(request)
	return &pb.Status{Ok: true}, nil
}

func (s server) RequestPremiere(ctx context.Context, request *pb.PremiereRequest) (*pb.Status, error) {
	if request.WorkerID != cfg.WorkerID {
		log.Info("Rejected request for premiere")
		return &pb.Status{Ok: false}, errors.New("unauthenticated: wrong worker id")
	}
	go worker.HandlePremiere(request)
	return &pb.Status{Ok: true}, nil
}

func (s server) RequestStreamEnd(ctx context.Context, request *pb.EndStreamRequest) (*pb.Status, error) {
	if request.WorkerID != cfg.WorkerID {
		log.Info("Rejected request to end stream")
		return &pb.Status{Ok: false}, errors.New("unauthenticated: wrong worker id")
	}
	go worker.HandleStreamEndRequest(request)
	return &pb.Status{Ok: true}, nil
}

func (s server) GenerateThumbnails(ctx context.Context, request *pb.GenerateThumbnailRequest) (*pb.Status, error) {
	if request.WorkerID != cfg.WorkerID {
		log.Info("Rejected request to generate thumbnails")
		return &pb.Status{Ok: false}, errors.New("unauthenticated: wrong worker id")
	}
	worker.HandleThumbnailRequest(request)
	return &pb.Status{Ok: true}, nil
}

func (s server) GenerateSectionImages(ctx context.Context, request *pb.GenerateSectionImageRequest) (*pb.GenerateSectionImageResponse, error) {
	// The course name is free text, so it is sanitized before it becomes a
	// directory name. Without this, a name containing path elements would make
	// the RemoveAll below delete a directory outside of the storage directory.
	folder, err := safepath.JoinInRoot(cfg.StorageDir,
		request.CourseName,
		fmt.Sprintf("%d.%s", request.CourseYear, request.CourseTeachingTerm),
		"sections")
	if err != nil {
		return &pb.GenerateSectionImageResponse{}, err
	}

	err = os.RemoveAll(folder) // clean up old section images
	if err != nil {
		return &pb.GenerateSectionImageResponse{}, err
	}
	err = os.MkdirAll(folder, 0o755) // make sure folder exists
	if err != nil {
		return &pb.GenerateSectionImageResponse{}, err
	}

	paths := make([]string, len(request.Sections))

	for i, section := range request.Sections {
		timestampStr := fmt.Sprintf("%0d:%0d:%0d", section.Hours, section.Minutes, section.Seconds)
		path := fmt.Sprintf("%s/%s.jpg", folder, uuid.NewUUID())

		cmd := exec.Command("ffmpeg", "-y",
			"-ss", timestampStr,
			"-i", request.PlaylistURL,
			"-vf", "scale=156:-1",
			"-frames:v", "1",
			"-q:v", "2",

			path)
		_, err = cmd.CombinedOutput()
		if err != nil {
			return &pb.GenerateSectionImageResponse{}, err
		}

		paths[i] = path
	}

	return &pb.GenerateSectionImageResponse{Paths: paths}, nil
}

func (s server) CombineThumbnails(ctx context.Context, request *pb.CombineThumbnailsRequest) (*pb.CombineThumbnailsResponse, error) {
	err := thumbgen.CombineThumbs(request.GetPrimaryThumbnail(), request.GetSecondaryThumbnail(), request.GetPath())
	if err != nil {
		return nil, err
	}
	return &pb.CombineThumbnailsResponse{FilePath: request.Path}, nil
}

// InitApi Initializes api endpoints
// addr: port to run on, e.g. ":8080"
func InitApi(addr string) {
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
	pb.RegisterToWorkerServer(grpcServer, &server{})

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
