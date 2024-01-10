package actions

import (
	"context"
	"log/slog"
)

func (a *ActionProvider) SelfStreamAction() *Action {
	return &Action{
		Type: SelfStreamAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {

			return ctx, nil
		},
	}
}
