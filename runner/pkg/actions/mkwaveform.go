package actions

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const (
	// waveformSampleRate keeps the decoded audio small: the levels are drawn a few hundred
	// pixels wide, so nothing above speech frequencies matters.
	waveformSampleRate = 8000
	// waveformBucketSeconds is the resolution of the timeline. The editor seeks in seconds.
	waveformBucketSeconds = 1
	// waveformFloorDB is the level that maps to 0, anything quieter is clipped to it.
	waveformFloorDB = -60
)

// waveformVersions are the versions the levels may be taken from, in order of preference. They
// have the same audio, so the combined one, which viewers watch, wins as for silences.
var waveformVersions = []string{"STREAM_VERSION_COMBINED", "STREAM_VERSION_CAMERA", "STREAM_VERSION_PRESENTATION"}

// MkWaveform builds the audio level timeline of a recorded stream and measures the length of
// each of its versions, and reports both in a WaveformReadyNotification.
//
// It reads the VODs from the playlists in d["sources"], by version, and not from its own
// storage: where a runner keeps what it writes is none of its business when it reads.
func MkWaveform(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	sources, _ := d["sources"].(map[string]string)
	source := ""
	for _, v := range waveformVersions {
		if sources[v] != "" {
			source = v
			break
		}
	}
	if source == "" {
		return AbortingError(fmt.Errorf("none of the stream's versions can be read: %d given", len(sources)))
	}

	release, err := acquirePostProcessing(ctx)
	if err != nil {
		return fmt.Errorf("waveform skipped, the job ended while it waited: %w", err)
	}
	defer release()

	n := &protobuf.WaveformReadyNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
		BucketSeconds: ptr.Take(uint32(waveformBucketSeconds)),
	}
	hasAudio, flatSeconds := true, 0.0
	for v, url := range sources {
		probe, err := ffmpeg.Probe(ctx, url)
		if err != nil || probe.Duration() <= 0 {
			// Left out rather than reported as 0: the server must not take a version it
			// couldn't measure for a short one.
			logger.Warn("couldn't measure version", "version", v, "err", err)
			continue
		}
		if v == source {
			hasAudio = probe.Codec("audio") != ""
			flatSeconds = probe.Duration()
		}
		n.Durations = append(n.Durations, &protobuf.WaveformReadyNotification_VersionDuration{
			StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[v])),
			Seconds:       ptr.Take(probe.Duration()),
		})
	}

	if !hasAudio {
		// Uploads and cuts take recordings without sound, and ffmpeg has no output to make
		// of one. A silent timeline still lets the editor seek and cut by the video.
		logger.Info("version has no audio, flat waveform", "version", source)
		n.Peaks = make([]byte, max(1, int(math.Ceil(flatSeconds/waveformBucketSeconds))))
	} else {
		logger.Info("building waveform", "version", source)
		peaks, err := decodePeaks(ctx, sources[source])
		if err != nil {
			return err
		}
		n.Peaks = peaks
	}
	notify <- &protobuf.Notification{Data: &protobuf.Notification_WaveformReady{WaveformReady: n}}
	return nil
}

func decodePeaks(ctx context.Context, input string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, silenceScanTimeout)
	defer cancel()
	args := []string{
		"-nostdin", "-nostats", "-v", "error", "-i", input, "-vn",
		"-ac", "1", "-ar", fmt.Sprint(waveformSampleRate), "-f", "s16le", "-",
	}
	name := "ffmpeg"
	// Niced for the same reason as the silence scan.
	if nice, err := exec.LookPath("nice"); err == nil {
		name, args = nice, append([]string{"ffmpeg"}, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 10 * time.Second
	w := newPeakWriter(waveformSampleRate * waveformBucketSeconds)
	var stderr tailBuffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, stderr.String())
	}
	return w.Peaks(), nil
}

// peakWriter turns signed 16 bit little endian mono samples into one level byte per bucket of
// samplesPerBucket samples: the peak of the bucket on a dBFS scale from waveformFloorDB to 0.
type peakWriter struct {
	samplesPerBucket int

	peaks   []byte
	peak    int // largest magnitude in the open bucket
	filled  int // samples in the open bucket
	odd     byte
	holdOdd bool // a chunk ended between the two bytes of a sample
}

func newPeakWriter(samplesPerBucket int) *peakWriter {
	return &peakWriter{samplesPerBucket: samplesPerBucket}
}

func (w *peakWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.holdOdd && len(p) > 0 {
		w.sample(int16(uint16(w.odd) | uint16(p[0])<<8))
		w.holdOdd, p = false, p[1:]
	}
	for ; len(p) >= 2; p = p[2:] {
		w.sample(int16(uint16(p[0]) | uint16(p[1])<<8))
	}
	if len(p) == 1 {
		w.odd, w.holdOdd = p[0], true
	}
	return n, nil
}

func (w *peakWriter) sample(s int16) {
	mag := int(s)
	if mag < 0 {
		mag = -mag
	}
	w.peak = max(w.peak, mag)
	w.filled++
	if w.filled == w.samplesPerBucket {
		w.flush()
	}
}

func (w *peakWriter) flush() {
	w.peaks = append(w.peaks, levelByte(w.peak))
	w.peak, w.filled = 0, 0
}

// Peaks returns the levels so far, with the bucket still open counted as a short one: the
// last second of a recording is rarely a whole one.
func (w *peakWriter) Peaks() []byte {
	if w.filled > 0 {
		w.flush()
	}
	return w.peaks
}

// levelByte maps a magnitude of 0 to 32768 on the scale of WaveformReadyNotification.peaks.
func levelByte(mag int) byte {
	if mag <= 0 {
		return 0
	}
	db := 20 * math.Log10(float64(mag)/32768)
	v := (db - waveformFloorDB) / -waveformFloorDB * 255
	return byte(math.Round(math.Min(math.Max(v, 0), 255)))
}
