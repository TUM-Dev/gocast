package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/u2takey/go-utils/uuid"

	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedToWorkerServer
}

// RequestCut is a gRPC endpoint for the worker to Cut a video
func (s server) RequestCut(ctx context.Context, request *pb.CutRequest) (*pb.CutResponse, error) {
	return nil, errors.New("not implemented")
}

// RequestWaveform is a gRPC endpoint for the worker to generate a waveform
func (s server) RequestWaveform(ctx context.Context, request *pb.WaveformRequest) (*pb.WaveFormResponse, error) {
	if request.WorkerId != cfg.WorkerID {
		return nil, errors.New("unauthenticated: wrong worker id")
	}
	waveform, err := worker.GetWaveform(request)
	return &pb.WaveFormResponse{Waveform: waveform}, err
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

// GenerateLiveThumbs generates a preview image of the most recent stream state.
func (s server) GenerateLivePreview(ctx context.Context, request *pb.LivePreviewRequest) (*pb.LivePreviewResponse, error) {
	if request.WorkerID != cfg.WorkerID {
		log.Info("Rejected request to generate live thumbnails")
		return nil, errors.New("unauthenticated: wrong worker id")
	}
	cmd := exec.Command("sh", "-c",
		"ffmpeg",
		"-sseof", "-3",
		"-i", request.HLSUrl,
		"-vframes", "1",
		"-update", "1",
		"-q:v", "1",
		"-c:v", "mjpeg",
		"-f", "mjpeg",
		"pipe:1")
	liveThumb, err := cmd.Output()
	return &pb.LivePreviewResponse{LiveThumb: liveThumb}, err
}

func (s server) GenerateSectionImages(ctx context.Context, request *pb.GenerateSectionImageRequest) (*pb.GenerateSectionImageResponse, error) {
	folder := fmt.Sprintf("%s/%s/%d.%s/sections",
		cfg.StorageDir, request.CourseName, request.CourseYear, request.CourseTeachingTerm)

	err := os.RemoveAll(folder) // clean up old section images
	if err != nil {
		return &pb.GenerateSectionImageResponse{}, err
	}
	err = os.MkdirAll(folder, os.ModePerm) // make sure folder exists
	if err != nil {
		return &pb.GenerateSectionImageResponse{}, err
	}

	paths := make([]string, len(request.Sections))

	for i, section := range request.Sections {
		timestampStr := fmt.Sprintf("%0d:%0d:%0d", section.Hours, section.Minutes, section.Seconds)
		path := fmt.Sprintf("%s/%s.jpg", folder, uuid.NewUUID())

		cmd := exec.Command("ffmpeg", "-y",
			"-ss", timestampStr,
			"-i", fmt.Sprintf("%s?jwt=%s", request.PlaylistURL, cfg.AdminToken),
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

func (s server) DeleteSectionImage(ctx context.Context, request *pb.DeleteSectionImageRequest) (*pb.Status, error) {
	err := os.RemoveAll(request.Path) // remove image file
	if err != nil {
		return &pb.Status{Ok: false}, err
	}
	return &pb.Status{Ok: true}, err
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
