package actions

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// PrepareAction prepares the directory structure for the stream and vod.
func (a *ActionProvider) PrepareAction() *Action {
	return &Action{
		Type: PrepareAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			streamID, ok := ctx.Value("stream").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain stream", ErrRequiredContextValNotFound)
			}
			courseID, ok := ctx.Value("course").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain course", ErrRequiredContextValNotFound)
			}
			version, ok := ctx.Value("version").(string)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain version", ErrRequiredContextValNotFound)
			}

			dirs := []string{
				a.GetRecDir(courseID, streamID, version),
				a.GetLiveDir(courseID, streamID, version),
				a.GetMassDir(courseID, streamID, version),
			}
			for _, dir := range dirs {
				log.Info("creating directory", "path", dir)
				err := os.MkdirAll(dir, 0755)
				if err != nil {
					return ctx, fmt.Errorf("create directory: %w", err)
				}
			}
			return ctx, nil
		},
	}
}
