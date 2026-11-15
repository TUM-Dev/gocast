// Package cutting is how apiv2 reaches the runners for the cut editor: the audio level
// timeline of a VOD and the cutting of it.
package cutting

import (
	"context"
	"errors"

	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// Cutting asks the runners to build waveforms and cut VODs. Only a runner can, so Manager
// being nil means there is nobody to ask.
type Cutting struct {
	Manager *runner_manager.Manager
}

// RequestWaveform asks a runner for the timeline of the stream and the length of the versions.
func (c Cutting) RequestWaveform(streamID uint, sources []apiv2.CutSource) error {
	if c.Manager == nil {
		return errors.New("no runner configured")
	}
	versions := make([]runner_manager.VersionSource, len(sources))
	for i, s := range sources {
		versions[i] = runner_manager.VersionSource{Version: s.Version, PlaylistURL: s.PlaylistURL}
	}
	return c.Manager.RequestWaveform(context.Background(), streamID, versions)
}

// Cut asks a runner to cut one version.
func (c Cutting) Cut(sourceStreamID, targetStreamID uint, source apiv2.CutSource, discard []model.CutRange) error {
	if c.Manager == nil {
		return errors.New("no runner configured")
	}
	return c.Manager.RequestCut(context.Background(), runner_manager.CutRequest{
		SourceStreamID: sourceStreamID, TargetStreamID: targetStreamID,
		Version: source.Version, PlaylistURL: source.PlaylistURL, Discard: discard,
	})
}

// ScanSilences asks a runner to detect the silences of the stream again.
func (c Cutting) ScanSilences(streamID uint) error {
	if c.Manager == nil {
		return errors.New("no runner configured")
	}
	return c.Manager.RequestSilenceScan(context.Background(), streamID)
}
