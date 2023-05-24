package actions

import (
	"context"
	"errors"
)

var (
	ErrActionInputWrongType       = errors.New("action input has wrong type")
	ErrRequiredContextValNotFound = errors.New("required context value not found")
)

const (
	recDir         = "/recordings/"
	liveSegmentDir = "/recordings/live/"
)

type Action func(ctx context.Context) (context.Context, error)

func set(ctx context.Context, key string, val interface{}) context.Context {
	return context.WithValue(ctx, key, val)
}
