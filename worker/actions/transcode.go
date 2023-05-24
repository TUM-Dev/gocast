package actions

import (
	"context"
	"os/exec"
)

func TranscodeAction(ctx context.Context) (context.Context, error) {
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
}
