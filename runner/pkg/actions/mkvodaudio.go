package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
)

// convertAudioVOD extracts the first audio track from a completed local or remote
// recording. audioDir must be a separate directory reserved for audio output.
// Missing audio returns false without changing a previous rendition; callers must
// invalidate its reference when the source video is replaced.
func convertAudioVOD(ctx context.Context, inputPlaylist, audioDir string) (bool, error) {
	audioDir = filepath.Clean(audioDir)
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

	// As with uploads, publish only after conversion succeeds. A retry starts with
	// an empty directory, so shorter replacements cannot leave old segments behind.
	if err := os.MkdirAll(filepath.Dir(audioDir), os.ModePerm); err != nil {
		return false, fmt.Errorf("create audio parent directory: %w", err)
	}
	workDir, err := os.MkdirTemp(filepath.Dir(audioDir), ".audio-")
	if err != nil {
		return false, fmt.Errorf("create audio work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }() // no-op once renamed

	args := []string{"-nostdin", "-y", "-v", "warning", "-xerror", "-abort_on", "empty_output", "-i", inputPlaylist, "-map", "0:a:0", "-vn"}
	if codec == "aac" {
		args = append(args, "-c:a", "copy")
	} else {
		// Historical recordings may use other codecs. AAC at 128 kbit/s keeps the
		// audio rendition compatible with the same browsers as current uploads.
		args = append(args, "-c:a", "aac", "-b:a", "128k")
	}
	args = append(args,
		"-f", "hls", "-hls_time", "20", "-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(workDir, "%05d.ts"),
		filepath.Join(workDir, "playlist.m3u8"),
	)
	command := exec.CommandContext(ctx, "ffmpeg", args...)
	command.WaitDelay = 10 * time.Second
	var stderr audioVODLog
	command.Stderr = &stderr
	err = command.Run()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, fmt.Errorf("convert audio VOD: %w: %s", err, stderr.String())
	}
	if stderr.skippedSegment {
		return false, fmt.Errorf("audio source has unreadable segments: %s", stderr.String())
	}
	if err := replaceVOD(workDir, audioDir); err != nil {
		return false, fmt.Errorf("publish audio VOD: %w", err)
	}
	return true, nil
}

// The HLS demuxer skips unreadable segments with a warning even with -xerror.
// Remember that warning independently of the diagnostic tail: publishing the
// remaining audio would lose part of the lecture and shift the playback timeline.
type audioVODLog struct {
	tailBuffer
	skippedSegment bool
	overlap        string
}

func (l *audioVODLog) Write(p []byte) (int, error) {
	const skipped = "failed too many times, skipping"
	text := l.overlap + string(p)
	l.skippedSegment = l.skippedSegment || strings.Contains(text, skipped)
	// A pipe write can split the warning anywhere, so keep enough for the next one.
	l.overlap = text[max(0, len(text)-len(skipped)+1):]
	return l.tailBuffer.Write(p)
}
