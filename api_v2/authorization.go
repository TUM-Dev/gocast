package api_v2

import (
	"context"
	"errors"
	"github.com/TUM-Dev/gocast/model"
	"google.golang.org/grpc/metadata"
)

// todo: implement this functionality
func (a *API) getUser(ctx context.Context) (*model.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}
	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return nil, errors.New("no authorization header")
	}
	return &model.User{Name: authHeader[0]}, nil
}
