package actions

import (
	"context"
	"github.com/tum-dev/gocast/runner/protobuf"
	"log/slog"
)

// Action represents a computation the runner executes.
//
// An action takes a context ctx, may cancel the action.
// Actions should use log for logging and notify for sending messages like their progress to gocast.
// d contains data passed to the action and is used to pass data to the next actions.
// Any error, the action returns will be logged. If that error is an AbortingError, the subsequent actions will be skipped.
type Action func(ctx context.Context, log *slog.Logger, notify chan *protobuf.Notification, d map[string]any) error
