// Package livepreview keeps the preview image of every livestream up to date.
package livepreview

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/pathprovider"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
})).With("service", "livepreview")

// LivePreviewer produces a JPEG of the most recent state of a livestream. The runner
// manager is the implementation; it is an interface so this package doesn't depend on it.
type LivePreviewer interface {
	RequestLivePreview(ctx context.Context, playlistURL string) ([]byte, error)
}

// Fetch gets a live thumbnail from a runner for every live stream.
func Fetch(daoWrapper dao.DaoWrapper, runners LivePreviewer) func() {
	return func() {
		liveStreams, err := daoWrapper.StreamsDao.GetCurrentLive(context.Background())
		if err != nil {
			return
		}

		// In case of an error, the preview might be outdated.
		// That's okay since the cron job runs again in a minute.
		for _, s := range liveStreams {
			if s.PlaylistUrl == "" {
				continue
			}
			if err := tools.SetSignedPlaylists(&s, nil, false); err != nil {
				logger.Error("Could not sign playlist for live preview", "err", err)
				continue
			}
			// The runner manager bounds the call itself.
			thumb, err := runners.RequestLivePreview(context.Background(), s.PlaylistUrl)
			if err != nil {
				logger.Error("Could not generate live preview", "stream", s.ID, "err", err)
				continue
			}
			if err := saveLivePreview(s.ID, thumb); err != nil {
				logger.Error("Could not save live preview", "stream", s.ID, "err", err)
			}
		}
	}
}

// saveLivePreview stores the preview where pathprovider.LiveThumbnail's readers look.
func saveLivePreview(streamID uint, thumb []byte) error {
	if err := os.MkdirAll(pathprovider.TUMLiveTemporary, 0o750); err != nil {
		return err
	}
	return os.WriteFile(pathprovider.LiveThumbnail(strconv.FormatUint(uint64(streamID), 10)), thumb, 0o644)
}
