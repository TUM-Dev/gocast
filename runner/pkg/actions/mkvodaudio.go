package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
)

// convertAudioVOD extracts the first audio track from a completed local or remote
// recording. audioDir must be a separate directory reserved for audio output.
// Missing audio returns false without changing a previous rendition; callers must
// invalidate its reference when the source video is replaced.
func convertAudioVOD(ctx context.Context, inputPlaylist, audioDir string) (bool, error) {
	probe, err := ffmpeg.Probe(ctx, inputPlaylist)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, fmt.Errorf("probe audio source: %w", err)
	}
	codec := probe.Codec("audio")
	if codec == "" {
		return false, nil
	}

	if err := os.MkdirAll(audioDir, os.ModePerm); err != nil {
		return false, fmt.Errorf("create audio directory: %w", err)
	}

	args := []string{"-nostdin", "-y", "-v", "error", "-xerror", "-abort_on", "empty_output", "-i", inputPlaylist, "-map", "0:a:0", "-vn"}
	if codec == "aac" {
		args = append(args, "-c:a", "copy")
	} else {
		// Historical recordings may use other codecs. AAC at 128 kbit/s keeps the
		// audio rendition compatible with the same browsers as current uploads.
		args = append(args, "-c:a", "aac", "-b:a", "128k")
	}
	args = append(args,
		"-f", "hls", "-hls_time", "20", "-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(audioDir, "%05d.ts"),
		filepath.Join(audioDir, "playlist.m3u8"),
	)
	command := exec.CommandContext(ctx, "ffmpeg", args...)
	command.WaitDelay = 10 * time.Second
	var stderr tailBuffer
	command.Stderr = &stderr
	err = command.Run()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, fmt.Errorf("convert audio VOD: %w: %s", err, stderr.String())
	}
	return true, nil
}
