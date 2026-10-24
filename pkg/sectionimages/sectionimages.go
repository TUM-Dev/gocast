// Package sectionimages is how apiv2 reaches section thumbnail generation.
package sectionimages

import (
	"context"
	"errors"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// SectionImages generates section thumbnails by asking a runner.
type SectionImages struct {
	// Nil when no runner manager is configured, which makes Generate fail.
	Manager *runner_manager.Manager
}

// Generate asks a runner for the thumbnails of sections of the recording at playlistURL,
// which must be signed. The runner answers asynchronously, so this returns once the job
// is accepted.
func (s SectionImages) Generate(streamID uint, playlistURL string, sections []model.VideoSection) error {
	if s.Manager == nil {
		return errors.New("no runner configured")
	}
	return s.Manager.RequestSectionImages(context.Background(),
		runner_manager.SectionImageRequest{StreamID: streamID, PlaylistURL: playlistURL, Sections: sections})
}
