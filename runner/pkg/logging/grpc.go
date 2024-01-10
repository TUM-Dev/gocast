package logging

import (
	"context"
	"google.golang.org/grpc"
	"log/slog"
)

// GetGrpcLogInterceptor returns a grpc.ServerOption that logs all requests
func GetGrpcLogInterceptor(logger *slog.Logger) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(grpcLogger(logger))
}

func grpcLogger(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		l.Info("gRPC call", "method", info.FullMethod, "request", req)
		return handler(ctx, req)
	}
}
