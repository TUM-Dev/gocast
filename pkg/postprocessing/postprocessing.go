// Package postprocessing is how apiv2 reaches the redoing of VOD post-processing:
// thumbnails and silences.
package postprocessing

import (
	"context"
	"errors"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// PostProcessing redoes the post-processing of one version of one VOD: its large thumbnail and,
// if asked, the silences. Only a
// runner can do this today, so Manager being nil means there is nobody to ask.
type PostProcessing struct {
	Manager *runner_manager.Manager
}

// Redo asks a runner to redo the thumbnail for one version of one VOD, and the
// silences of the stream if detectSilence is set. playlistURL must be signed.
func (t PostProcessing) Redo(streamID uint, version model.StreamVersion, playlistURL string, detectSilence bool) error {
	if t.Manager == nil {
		return errors.New("no runner configured")
	}
	return t.Manager.RequestRegenerateThumbnail(context.Background(), runner_manager.RegenerateThumbnailRequest{
		StreamID:      streamID,
		Version:       version,
		PlaylistURL:   playlistURL,
		DetectSilence: detectSilence,
	})
}
