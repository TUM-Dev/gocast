package worker

import (
	"github.com/joschahenningsen/thumbgen"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	ThumbWidth  = 160 // Width in pixels, height is inferred by thumbgen
	Compression = 90  // Compression in percent
)

// createThumbnailSprite creates a thumbnail sprite from the given video file and stores it in mass storage.
func createThumbnailSprite(ctx *StreamContext, source string) error {
	secondsPerHour := uint32(time.Hour.Seconds())
	switch {
	case ctx.duration < secondsPerHour:
		ctx.thumbInterval = 10
	case ctx.duration > secondsPerHour*3:
		ctx.thumbInterval = 60
	default:
		ctx.thumbInterval = 30
	}
	log.WithField("File", ctx.getThumbnailSpriteFileName()).Info("Run creating thumbnail sprite")
	g, err := thumbgen.New(source, ThumbWidth, int(ctx.thumbInterval), ctx.getThumbnailSpriteFileName(), thumbgen.WithJpegCompression(Compression))
	if err != nil {
		return err
	}
	err = g.Generate()
	log.WithField("file", ctx.getThumbnailSpriteFileName()).Info("Finished creating thumbnail sprite")
	return err
}
