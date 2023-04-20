package worker

import (
	"github.com/joschahenningsen/thumbgen"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"time"
)

const (
	ThumbWidth      = 160 // Width in pixels, height is inferred by thumbgen
	LargeThumbWidth = 720
	Compression     = 90 // Compression in percent
)

// createThumbnailSprite creates a thumbnail sprite from the given video file for the seekbar and stores it in mass storage.
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
	log.WithField("File", ctx.getThumbnailSpriteFileName()).Info("Start creating thumbnail sprite")
	g, err := thumbgen.New(source, ThumbWidth, int(ctx.thumbInterval), ctx.getThumbnailSpriteFileName(), thumbgen.WithJpegCompression(Compression))
	if err != nil {
		return err
	}
	err = g.Generate()
	log.WithField("file", ctx.getThumbnailSpriteFileName()).Info("Finished creating thumbnail sprite")
	return err
}

// createVideoThumbnail creates a thumbnail from the given video file and stores it in mass storage.
func createVideoThumbnail(ctx *StreamContext, source string) error {
	g, err := thumbgen.New(source, LargeThumbWidth, 1, "", thumbgen.WithJpegCompression(Compression))
	if err != nil {
		return err
	}
	path, err := g.GenerateOne()
	if err != nil {
		return err
	}
	thumb, err := os.Open(path)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(ctx.getLargeThumbnailSpriteFileName(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(thumb, file)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil {
		log.WithError(err).Warn("Could not remove temporary thumbnail file")
	}
	return nil
}
