// Package thumbnails is how apiv2 reaches VOD thumbnail regeneration.
package thumbnails

import (
	"context"
	"errors"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// Thumbnails regenerates the large thumbnail of one version of one VOD. Only a
// runner can do this today, so Manager being nil means there is nobody to ask.
type Thumbnails struct {
	Manager *runner_manager.Manager
}

// Regenerate asks a runner to redo the thumbnail for one version of one VOD.
// playlistURL must be signed.
func (t Thumbnails) Regenerate(streamID uint, version model.StreamVersion, playlistURL string) error {
	if t.Manager == nil {
		return errors.New("no runner configured")
	}
	return t.Manager.RequestRegenerateThumbnail(context.Background(), runner_manager.RegenerateThumbnailRequest{
		StreamID:    streamID,
		Version:     version,
		PlaylistURL: playlistURL,
	})
}
