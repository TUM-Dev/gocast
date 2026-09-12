package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const (
	// sectionImageWidth is the width of a section thumbnail in pixels, the height
	// follows from the aspect ratio of the recording.
	sectionImageWidth = 156
	// sectionImageQuality is the ffmpeg -q:v value used for the generated jpegs,
	// where 2 is the best quality the mjpeg encoder offers.
	sectionImageQuality = 2
)

// SectionTimestamp identifies one video section and the offset into the recording it starts at.
type SectionTimestamp struct {
	SectionID uint64
	Hours     uint32
	Minutes   uint32
	Seconds   uint32
}

// MkSectionImages generates a thumbnail for every video section of a recording and
// sends them to gocast in a notification.
//
// The images are passed on as bytes rather than written to disk here: gocast owns
// the mass storage directory and decides where they end up, so the runner never
// builds a storage path out of course metadata.
func MkSectionImages(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	playlistURL, ok := d["playlistURL"].(string)
	if !ok || playlistURL == "" {
		return AbortingError(fmt.Errorf("no playlist url in context"))
	}
	sections, ok := d["sections"].([]SectionTimestamp)
	if !ok {
		return AbortingError(fmt.Errorf("no sections in context"))
	}
	if len(sections) == 0 {
		logger.Info("no sections to generate images for")
		return nil
	}

	logger.Info("generating section images", "sections", len(sections))

	images := make([]*protobuf.SectionImage, 0, len(sections))
	for _, section := range sections {
		image, err := createSectionImage(ctx, playlistURL, section)
		if err != nil {
			// One unreadable timestamp should not cost us the remaining sections.
			logger.Error("error creating section image", "sectionID", section.SectionID, "error", err)
			continue
		}
		images = append(images, &protobuf.SectionImage{
			SectionId: ptr.Take(section.SectionID),
			Image:     image,
		})
	}

	if len(images) == 0 {
		return AbortingError(fmt.Errorf("could not generate any of the %d section images", len(sections)))
	}

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_SectionImagesReady{SectionImagesReady: &protobuf.SectionImagesReadyNotification{
			Stream: &protobuf.StreamInfo{Id: ptr.Take(streamID)},
			Images: images,
		}},
	}
	return nil
}

// createSectionImage extracts a single frame at the sections timestamp and returns it as jpeg.
func createSectionImage(ctx context.Context, playlistURL string, section SectionTimestamp) ([]byte, error) {
	timestamp := fmt.Sprintf("%02d:%02d:%02d", section.Hours, section.Minutes, section.Seconds)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-ss", timestamp,
		"-i", playlistURL,
		"-vf", fmt.Sprintf("scale=%d:-1", sectionImageWidth),
		"-frames:v", "1",
		"-q:v", fmt.Sprintf("%d", sectionImageQuality),
		"-f", "mjpeg",
		"pipe:1")
	cmd.Stderr = &stderr

	image, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg at %s: %w (%s)", timestamp, err, stderr.String())
	}
	if len(image) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no image at %s", timestamp)
	}
	return image, nil
}
