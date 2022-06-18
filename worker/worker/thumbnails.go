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
func createThumbnailSprite(ctx *StreamContext) error {
	var ThumbInterval int // Specifies the interval between thumbnails in seconds.
	secondsPerHour := uint32(time.Hour.Seconds())
	switch {
	case ctx.duration < secondsPerHour:
		ThumbInterval = 10
	case ctx.duration > secondsPerHour*3:
		ThumbInterval = 60
	default:
		ThumbInterval = 30
	}
	log.WithField("File", ctx.getThumbnailSpriteFileName()).Info("Start creating thumbnail sprite")
	g, err := thumbgen.New(ctx.getTranscodingFileName(), ThumbWidth, ThumbInterval, ctx.getThumbnailSpriteFileName(), thumbgen.WithJpegCompression(Compression))
	if err != nil {
		return err
	}
	err = g.Generate()
	log.WithField("file", ctx.getThumbnailSpriteFileName()).Info("Finished creating thumbnail sprite")
	return err
}
