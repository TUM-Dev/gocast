package runner_manager

import (
	"context"
	"fmt"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// livePreviewTimeout has to stay well below the one minute preview cron period. It
// includes dialing, since dialRunner is lazy.
const livePreviewTimeout = 15 * time.Second

// RequestLivePreview asks a runner for a JPEG of the most recent state of a livestream.
// playlistURL must be signed. Unlike the other thumbnail requests this one is
// synchronous, since a preview is replaced a minute later anyway.
func (m *Manager) RequestLivePreview(ctx context.Context, playlistURL string) ([]byte, error) {
	_, client, conn, err := m.getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(ctx, livePreviewTimeout)
	defer cancel()

	resp, err := client.GenerateLivePreview(ctx, &protobuf.LivePreviewRequest{PlaylistUrl: ptr.Take(playlistURL)})
	if err != nil {
		return nil, fmt.Errorf("GenerateLivePreview: %w", err)
	}
	return resp.GetThumbnail(), nil
}
