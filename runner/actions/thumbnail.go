package actions

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/thumbgen"
	"io"
	"log/slog"
	"os"
)

const (
	//ThumbWidth      = 160
	LargeThumbWidth = 720
	Compression     = 90
)

// GenerateVideoThumbnail generate a Thumbnail from the stream the runner just run.
// it will also check if it was the first to generate a thumbnail for the stream. if it wasn't,
// it will combine the two generated thumbnails into one/*
func (a *ActionProvider) GenerateVideoThumbnail() *Action {
	return &Action{
		Type: ThumbnailAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			pathForThumb := a.MassDir
			pathForRec := a.RecDir
			nameOfFile := fmt.Sprintf("%x-thumb-1", ctx.Value(""))
			g, err := thumbgen.New(pathForRec, LargeThumbWidth, 1, "", thumbgen.WithJpegCompression(Compression))
			if err != nil {
				log.Error("couldn't generate new thumbnail generator")
				return ctx, err
			}
			file, err := g.GenerateOne()
			if err != nil {
				log.Error("couldn't create a Thumbnail in the middle of the Stream")
				return ctx, err
			}
			thumb, err := os.Open(file)
			if err != nil {
				log.Error("couldn't open the Thumbnail")
				return ctx, err
			}
			if _, err := os.Stat(fmt.Sprintf("%x", pathForThumb)); os.IsNotExist(err) {
				openFile, err := os.OpenFile(fmt.Sprintf("%x/%x", pathForThumb, nameOfFile), os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Error("couldn't open the file for the Thumbnail")
					return ctx, err
				}
				defer openFile.Close()
				_, err = io.Copy(openFile, thumb)
				if err != nil {
					log.Error("couldn't copy the Thumbnail to the file")
					return ctx, err
				}
				err = os.Remove(file)
				if err != nil {
					log.Error("couldn't remove the Thumbnail")
				}
			} else {
				thumpOne := fmt.Sprintf("%x/%x", pathForThumb, nameOfFile)
				nameOfFile := fmt.Sprintf("%x-2", nameOfFile)
				openFile, err := os.OpenFile(fmt.Sprintf("%x/%x", pathForThumb, nameOfFile), os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Error("couldn't open the file for the Thumbnail")
					return ctx, err
				}
				defer openFile.Close()
				_, err = io.Copy(openFile, thumb)
				if err != nil {
					log.Error("couldn't copy the Thumbnail to the file")
					return ctx, err
				}
				err = os.Remove(file)
				if err != nil {
					log.Error("couldn't remove the Thumbnail")
				}
				err = thumbgen.CombineThumbs(thumpOne, nameOfFile, pathForThumb)
				if err != nil {
					log.Error("couldn't combine the Thumbnails")
				}
			}

			return ctx, nil
		},
	}
}

func (a *ActionProvider) GenerateThumbnailSprite() *Action {
	return &Action{
		Type: ThumbnailAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			return ctx, nil
		},
	}
}
