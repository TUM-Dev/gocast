package actions

import (
	"context"
	"log/slog"
	"os/exec"
	"time"
)

func (a *ActionProvider) TranscodeAction() *Action {
	return &Action{
		Type: TranscodeAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			files, ok := ctx.Value("files").([]string)
			if !ok {
				return ctx, ErrActionInputWrongType
			}
			log.Info("transcoding", "files", files)
			time.Sleep(time.Second)
			return ctx, nil
			// parse output from previous streamAction
			fileName, ok := ctx.Value("files").([]string)
			if !ok {
				return ctx, ErrActionInputWrongType
			}
			_ = "/mass/" + ctx.Value("streamID").(string) + ".mp4"
			c := exec.CommandContext(ctx, "ffmpeg", fileName...) //, "...", output)
			err := c.Start()
			if err != nil {
				return ctx, err
			}
			err = c.Wait()
			return ctx, err
		},
	}
}
