package worker

import (
	"github.com/joschahenningsen/thumbgen"
	log "github.com/sirupsen/logrus"
)

const (
	ThumbCount  = 600 // How many thumbnails to generate, we use a static size, so we have a deterministic file size
	ThumbWidth  = 200 // Width in pixels, height is inferred by thumbgen
	Compression = 90  // Compression in percent
)

// createThumbnailSprite creates a thumbnail sprite from the given video file and stores it in mass storage.
func createThumbnailSprite(ctx *StreamContext) error {
	log.WithField("File", ctx.getThumbnailSpriteFileName()).Info("Start creating thumbnail sprite")
	g, err := thumbgen.New(ctx.getTranscodingFileName(), ThumbWidth, ThumbCount, ctx.getThumbnailSpriteFileName(), thumbgen.WithJpegCompression(Compression))
	if err != nil {
		return err
	}
	err = g.Generate()
	log.WithField("file", ctx.getThumbnailSpriteFileName()).Info("Finished creating thumbnail sprite")
	return err
}
