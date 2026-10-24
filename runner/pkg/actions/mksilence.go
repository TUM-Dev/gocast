package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const (
	// silenceThresholdDB is the level below which audio counts as silent.
	silenceThresholdDB = -15
	// minSilenceSeconds is the shortest stretch reported, and also the shortest sound that
	// breaks one silence into two: the player offers to skip a silence, which is only worth
	// it for a long one, and a cough in the middle should not split it.
	minSilenceSeconds = 30
)

// Silence is a stretch of a video without sound, in whole seconds from its start.
type Silence struct {
	Start, End uint32
}

// MkSilence scans the VoD for silences and reports them in a SilenceDetectedNotification.
//
// A failed scan is logged and otherwise ignored. Silences are a convenience for viewers, so
// it must neither fail the job nor be retried: the scan decodes the whole audio track.
func MkSilence(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	sv, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	// A recorded stream leaves its VoD at vodPath, an import at playlistURL. An empty one
	// is as good as missing: ffmpeg would only fail on it, and quietly.
	input, _ := d["vodPath"].(string)
	if input == "" {
		input, _ = d["playlistURL"].(string)
	}
	if input == "" {
		recordingDir, ok := d["recordingDir"].(string)
		if !ok {
			return AbortingError(fmt.Errorf("none of vodPath, playlistURL and recordingDir in context"))
		}
		input = path.Join(recordingDir, "playlist.m3u8")
	}

	// Waited for before the scan's own timeout starts, which is only for the scan.
	release, err := acquirePostProcessing(ctx)
	if err != nil {
		logger.Error("silence detection skipped, the job ended while it waited", "err", err)
		return nil
	}
	defer release()

	logger.Info("detecting silence", "input", input)
	silences, err := detectSilences(ctx, input)
	if err != nil {
		logger.Error("silence detection failed", "err", err)
		return nil
	}
	// No silence is a result too, and sent: the manager replaces what it has, which for a
	// redone scan may be silences that are gone.
	n := &protobuf.SilenceDetectedNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
		StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[sv])),
	}
	for _, s := range silences {
		n.Starts = append(n.Starts, s.Start)
		n.Ends = append(n.Ends, s.End)
	}
	notify <- &protobuf.Notification{Data: &protobuf.Notification_SilenceDetected{SilenceDetected: n}}
	return nil
}

// silenceScanTimeout bounds one scan. It decodes the whole audio track, often of a remote
// playlist, and a stalled connection would otherwise hold the runner for good.
const silenceScanTimeout = time.Hour

func detectSilences(ctx context.Context, input string) ([]Silence, error) {
	ctx, cancel := context.WithTimeout(ctx, silenceScanTimeout)
	defer cancel()
	// -vn: only the audio matters, so skip decoding the video.
	args := []string{"-nostdin", "-nostats", "-i", input, "-vn",
		"-af", fmt.Sprintf("silencedetect=n=%ddB:d=%d", silenceThresholdDB, minSilenceSeconds), "-f", "null", "-"}
	name := "ffmpeg"
	// Niced as the worker did: a scan of a long lecture runs for minutes next to the
	// recordings and transcodes of the same runner, which must not lose frames to it.
	if nice, err := exec.LookPath("nice"); err == nil {
		name, args = nice, append([]string{"ffmpeg"}, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, tail(out.String(), 500))
	}
	return mergeSilences(parseSilenceOutput(out.String())), nil
}

// tail returns at most the last n bytes of s, which is where ffmpeg reports why it failed.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// parseSilenceOutput reads the silence_start / silence_end lines of ffmpeg's silencedetect
// filter. An end without a start is ignored. A start without an end, which is how a silence
// that runs to the end of the video can look, is dropped: there is nothing left to skip to.
func parseSilenceOutput(out string) []Silence {
	var silences []Silence
	open := false
	for _, line := range strings.Split(out, "\n") {
		if _, v, ok := strings.Cut(line, "silence_start:"); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				continue
			}
			silences = append(silences, Silence{Start: toSeconds(f)})
			open = true
		} else if _, v, ok := strings.Cut(line, "silence_end:"); ok && open {
			v, _, _ = strings.Cut(v, "|")
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				continue
			}
			silences[len(silences)-1].End = toSeconds(f)
			open = false
		}
	}
	// A trailing silence has no end line; drop it rather than invent an end.
	if open {
		silences = silences[:len(silences)-1]
	}
	return silences
}

// toSeconds truncates to whole seconds. ffmpeg reports a slightly negative start for
// silence at the very beginning.
func toSeconds(f float64) uint32 {
	if f < 0 {
		return 0
	}
	return uint32(f)
}

// mergeSilences joins silences that are less than minSilenceSeconds apart, so that short
// sounds inside a long silence do not split it, and lets a silence that begins within the
// first seconds start at 0: the player only auto-skips an intro silence that starts there.
func mergeSilences(in []Silence) []Silence {
	if len(in) == 0 {
		return nil
	}
	merged := []Silence{in[0]}
	for _, s := range in[1:] {
		last := &merged[len(merged)-1]
		if int64(s.Start)-int64(last.End) < minSilenceSeconds {
			last.End = max(last.End, s.End)
			continue
		}
		merged = append(merged, s)
	}
	if merged[0].Start < minSilenceSeconds {
		merged[0].Start = 0
	}
	return merged
}
