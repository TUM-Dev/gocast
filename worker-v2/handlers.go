package worker

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/worker-v2/pb"
)

// RequestCut is a gRPC endpoint for the worker to Cut a video
func (w *Worker) RequestCut(ctx context.Context, r *pb.CutRequest) (*pb.CutResponse, error) {
	return nil, nil
}

// RequestWaveform is a gRPC endpoint for the worker to generate a waveform
func (w *Worker) RequestWaveform(ctx context.Context, r *pb.WaveformRequest) (*pb.WaveFormResponse, error) {
	return &pb.WaveFormResponse{}, nil
}

func (w *Worker) RequestStream(ctx context.Context, r *pb.StreamRequest) (*pb.Status, error) {
	return &pb.Status{}, nil
}

func (w *Worker) RequestPremiere(ctx context.Context, r *pb.PremiereRequest) (*pb.Status, error) {
	return &pb.Status{Ok: true}, nil
}

func (w *Worker) RequestStreamEnd(ctx context.Context, r *pb.EndStreamRequest) (*pb.Status, error) {
	return &pb.Status{Ok: true}, nil
}

func (w *Worker) GenerateThumbnails(ctx context.Context, r *pb.GenerateThumbnailRequest) (*pb.Status, error) {
	return &pb.Status{Ok: true}, nil
}

// GenerateLivePreview generates a preview image of the most recent stream state.
func (w *Worker) GenerateLivePreview(ctx context.Context, r *pb.LivePreviewRequest) (*pb.LivePreviewResponse, error) {
	return &pb.LivePreviewResponse{}, nil
}

func (w *Worker) GenerateSectionImages(ctx context.Context, r *pb.GenerateSectionImageRequest) (*pb.GenerateSectionImageResponse, error) {
	return &pb.GenerateSectionImageResponse{}, nil
}

func (w *Worker) DeleteSectionImage(ctx context.Context, r *pb.DeleteSectionImageRequest) (*pb.Status, error) {
	return &pb.Status{Ok: true}, nil
}
