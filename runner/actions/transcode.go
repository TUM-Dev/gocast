package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
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
			if files == nil {
				log.Error("no files to transcode", "files", files)
				return ctx, ErrRequiredContextValNotFound
			}
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

			log.Info("transcoding", "files", files)
			time.Sleep(time.Second)
			// parse output from previous streamAction
			fileName, ok := ctx.Value("files").([]string)
			if !ok {
				return ctx, ErrActionInputWrongType
			}
			filenames := ""
			if len(fileName) == 1 {
				filenames = fileName[0]
			} else {
				filenames = `"concat:` + fileName[0]
				for i := 1; i < len(fileName); i++ {
					filenames += "|" + fileName[i]
				}
				filenames += `"`
			}

			outputName := a.GetMassDir(courseID, streamID, version) + "/" + time.Now().Format("2006-01-02") + ".mp4"
			i := 1
			_, err := os.Stat(outputName)
			for err == nil {
				if errors.Is(err, os.ErrNotExist) {
					break
				}
				outputName = fmt.Sprintf(a.GetMassDir(courseID, streamID, version)+"/"+time.Now().Format("2006-01-02")+"_%d"+".mp4", i)
				_, err = os.Stat(outputName)
				i++
			}

			cmd := fmt.Sprintf(a.Cmd.Transcoding, filenames, outputName)
			c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
			c.Stderr = os.Stderr
			err = c.Start()
			if err != nil {
				return ctx, err
			}
			err = c.Wait()
			return ctx, err
		},
	}
}
