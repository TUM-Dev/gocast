package actions

import (
	"context"
	"log/slog"
)

func (a *ActionProvider) UploadAction() *Action {
	return &Action{
		Type: UploadAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			return ctx, nil
		},
	}
}
