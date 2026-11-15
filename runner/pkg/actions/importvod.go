package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// maxCopyLevel is the highest h264 level (4.2) that browsers decode reliably. Uploads
// above it are transcoded even if the codec is right.
const maxCopyLevel = 42

// importError carries what to tell the person who uploaded the video next to the
// error for the log: the error names paths and ffmpeg output that are none of their
// business.
type importError struct {
	reason string
	err    error
}

func (e *importError) Error() string { return e.err.Error() }
func (e *importError) Unwrap() error { return e.err }

// failImport is an aborting error that VODFailedNotification reports with reason.
func failImport(reason string, err error) error {
	return AbortingError(&importError{reason: reason, err: err})
}

const (
	reasonUnreadable  = "The file could not be read as a video."
	reasonNoVideo     = "The file contains no video."
	reasonConversion  = "The video could not be converted."
	reasonStorage     = "The video could not be stored."
	reasonInterrupted = "The conversion was interrupted. Please upload the video again."
	reasonOther       = "Importing the video failed."
)

// ImportVOD turns an uploaded video file into a VOD: it is repackaged into HLS without
// re-encoding when browsers can play it as is, and transcoded otherwise. The result
// lands where MkVOD puts the VODs of recorded streams, so the edge serves both alike.
//
// The uploaded file is removed afterwards, whether or not the import worked. Every
// failure is aborting: retrying a file ffmpeg rejected once only burns CPU.
func ImportVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	err := importVOD(ctx, logger, notify, d, metrics)
	if err == nil {
		return nil
	}
	// Without these there is no import to report on: a job that was set up wrong.
	streamID, ok1 := d["streamID"].(uint64)
	streamVersion, ok2 := d["streamVersion"].(string)
	if file, _ := d["uploadPath"].(string); ok1 && ok2 && file != "" {
		reason := reasonOther
		var ie *importError
		switch {
		case ctx.Err() != nil:
			reason = reasonInterrupted
		case errors.As(err, &ie):
			reason = ie.reason
		}
		// The uploader would otherwise wait for a video that is never coming.
		notify <- &protobuf.Notification{
			Data: &protobuf.Notification_VodFailed{
				VodFailed: &protobuf.VODFailedNotification{
					Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
					StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
					Reason:        ptr.Take(reason),
				},
			},
		}
	}
	return err
}

func importVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	uploadPath, ok := d["uploadPath"].(string)
	if !ok || uploadPath == "" {
		return AbortingError(fmt.Errorf("no upload path in context"))
	}
	// A browser recording's segments, of which uploadPath is the first.
	inputs := []string{uploadPath}
	if paths, ok := d["uploadPaths"].([]string); ok && len(paths) > 1 {
		inputs = paths
	}
	defer func() {
		for _, file := range inputs {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				logger.Warn("couldn't remove uploaded file", "file", file, "err", err)
			}
		}
	}()

	metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Inc()
	defer metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Dec()

	probes := make([]*ffmpeg.FFProbeResult, 0, len(inputs))
	for _, file := range inputs {
		probe, err := ffmpeg.Probe(ctx, file)
		if err != nil {
			return failImport(reasonUnreadable, fmt.Errorf("probe upload: %w", err))
		}
		if probe.Codec("video") == "" {
			return failImport(reasonNoVideo, fmt.Errorf("upload has no video stream"))
		}
		probes = append(probes, probe)
	}

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	// Written next to the final directory and swapped in at the end: a failed re-upload
	// must leave the previous VOD intact, and a half-written one must never be served.
	// Unique per import, so two uploads of the same stream version can't wipe each
	// other's output.
	if err := os.MkdirAll(path.Dir(vodDir), os.ModePerm); err != nil {
		return failImport(reasonStorage, fmt.Errorf("create stream directory: %w", err))
	}
	workDir, err := os.MkdirTemp(path.Dir(vodDir), ".importing-")
	if err != nil {
		return failImport(reasonStorage, fmt.Errorf("create work directory: %w", err))
	}
	defer func() { _ = os.RemoveAll(workDir) }() // no-op once renamed

	var args []string
	if len(inputs) == 1 {
		probe := probes[0]
		transcode := needsTranscode(probe.Codec("video"), probe.Codec("audio"), probe.Level(), probe.PixelFormat())
		logger.Info("importing upload", "transcode", transcode, "file", uploadPath)
		args = importArgs(uploadPath, workDir, transcode)
	} else {
		width, height := probes[0].Dimensions()
		audio := true
		for _, p := range probes {
			audio = audio && p.Codec("audio") != ""
		}
		if !audio && probes[0].Codec("audio") != "" {
			logger.Warn("joining segments of which some have no audio, dropping it", "segments", len(inputs))
		}
		logger.Info("joining uploaded segments", "segments", len(inputs), "width", width, "height", height, "audio", audio)
		args = concatArgs(inputs, workDir, width, height, audio)
	}
	// An argument slice, not a split string: the file name comes from the uploader.
	command := exec.CommandContext(ctx, "ffmpeg", args...)
	command.WaitDelay = 10 * time.Second
	// Not StderrPipe: Wait closes the pipe, so reading it concurrently can lose the end
	// of the output, which is where ffmpeg says why it failed.
	var stderr tailBuffer
	command.Stderr = &stderr
	// ffmpeg reports how far it is on stdout. A writer rather than a pipe for the reason
	// above: Wait returns only once it has been handed everything.
	command.Stdout = newProgressWriter(totalDuration(probes), progressReporter(notify, streamID, streamVersion))
	if err := command.Run(); err != nil {
		return failImport(reasonConversion, fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String()))
	}

	if err := replaceVOD(workDir, vodDir); err != nil {
		return failImport(reasonStorage, err)
	}

	vodPath := path.Join(vodDir, "playlist.m3u8")
	// The thumbnail action reads the VOD from here.
	d["playlistURL"] = vodPath

	vodURL, err := url.JoinPath(config.Config.EdgeServer, "vod", fmt.Sprintf("%d", streamID), streamVersion, "playlist.m3u8")
	if err != nil {
		return failImport(reasonOther, fmt.Errorf("join vod url: %w", err))
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_VodReady{
			VodReady: &protobuf.VODReadyNotification{
				Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
				StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
				Url:           ptr.Take(vodURL),
			},
		},
	}
	return nil
}

// needsTranscode reports whether an upload can't be repackaged as is. The container
// doesn't matter, only the codecs do: HLS segments are rewritten either way. A missing
// audio track is fine, a missing level is not, as the video can't be shown to be playable.
// The pixel format matters too: 10 bit and 4:2:2/4:4:4 H.264, which cameras and screen
// recorders produce, have the right codec name and the right level, yet browsers can't
// decode them.
func needsTranscode(videoCodec, audioCodec, level, pixFmt string) bool {
	if videoCodec != "h264" {
		return true
	}
	if pixFmt != "yuv420p" && pixFmt != "yuvj420p" {
		return true
	}
	if audioCodec != "" && audioCodec != "aac" {
		return true
	}
	l, err := strconv.Atoi(level)
	return err != nil || l > maxCopyLevel
}

// importArgs builds the ffmpeg arguments that write input as a VOD playlist into vodDir.
func importArgs(input, vodDir string, transcode bool) []string {
	args := []string{"-nostdin", "-y", "-nostats", "-progress", "pipe:1", "-i", input}
	if transcode {
		args = append(args,
			// Even dimensions, as yuv420p requires them.
			"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
			// Keyframes on the segment boundaries keep the segments close to hls_time.
			"-force_key_frames", "expr:gte(t,n_forced*20)",
			"-c:a", "aac",
		)
	} else {
		args = append(args, "-c", "copy")
	}
	return append(args, hlsOutputArgs(vodDir)...)
}

// concatArgs builds the ffmpeg arguments that join the segments of a browser recording,
// in order, into one VOD playlist in vodDir. Always transcoded: the timestamps of each
// segment start at zero, and segments can differ in size, as a screen shared anew does.
// Each is fitted into the first one's size, which the concat filter requires. So does
// having the same streams in each: audio is kept only if every segment has it.
func concatArgs(inputs []string, vodDir string, width, height int, audio bool) []string {
	if width <= 0 || height <= 0 {
		width, height = 1920, 1080
	}
	// Even dimensions, as yuv420p requires them.
	width, height = width/2*2, height/2*2

	args := []string{"-nostdin", "-y", "-nostats", "-progress", "pipe:1"}
	for _, in := range inputs {
		args = append(args, "-i", in)
	}
	var filter, joined strings.Builder
	for i := range inputs {
		fmt.Fprintf(&filter, "[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p[v%d];",
			i, width, height, width, height, i)
		fmt.Fprintf(&joined, "[v%d]", i)
		if audio {
			fmt.Fprintf(&filter, "[%d:a]aresample=48000,aformat=channel_layouts=stereo[a%d];", i, i)
			fmt.Fprintf(&joined, "[a%d]", i)
		}
	}
	a := 0
	if audio {
		a = 1
	}
	fmt.Fprintf(&filter, "%sconcat=n=%d:v=1:a=%d[v]", joined.String(), len(inputs), a)
	if audio {
		filter.WriteString("[a]")
	}

	args = append(args, "-filter_complex", filter.String(), "-map", "[v]")
	if audio {
		args = append(args, "-map", "[a]", "-c:a", "aac")
	}
	args = append(args,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		// Keyframes on the segment boundaries keep the segments close to hls_time.
		"-force_key_frames", "expr:gte(t,n_forced*20)",
	)
	return append(args, hlsOutputArgs(vodDir)...)
}

// totalDuration is the length of the inputs together, or 0 when one has none: a
// MediaRecorder's WebM has no duration in its header, and progress measured against
// part of the length would be wrong.
func totalDuration(probes []*ffmpeg.FFProbeResult) float64 {
	var total float64
	for _, p := range probes {
		d := p.Duration()
		if d <= 0 {
			return 0
		}
		total += d
	}
	return total
}

// hlsOutputArgs are the arguments that write the encoded streams as a VOD playlist into vodDir.
func hlsOutputArgs(vodDir string) []string {
	return []string{
		"-f", "hls",
		"-hls_time", "20",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", path.Join(vodDir, "%05d.ts"),
		path.Join(vodDir, "playlist.m3u8"),
	}
}

// replaceVOD puts the finished import at vodDir, replacing a previous VOD. The old one is
// moved aside rather than deleted first, so a failure leaves it in place, and viewers
// only miss the playlist for the time between two renames.
func replaceVOD(workDir, vodDir string) error {
	var trash string
	if _, err := os.Stat(vodDir); err == nil {
		trash, err = os.MkdirTemp(path.Dir(vodDir), ".replaced-")
		if err != nil {
			return fmt.Errorf("create trash directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(trash) }()
		if err := os.Rename(vodDir, path.Join(trash, "vod")); err != nil {
			return fmt.Errorf("move previous VOD aside: %w", err)
		}
	}
	if err := os.Rename(workDir, vodDir); err != nil {
		if trash != "" {
			_ = os.Rename(path.Join(trash, "vod"), vodDir) // best effort: keep the old VOD
		}
		return fmt.Errorf("publish VOD: %w", err)
	}
	return nil
}

// tailBuffer keeps the last bytes written to it: the end of ffmpeg's output is the part
// that holds the error.
type tailBuffer struct{ b []byte }

const tailBufferSize = 4096

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > tailBufferSize {
		t.b = t.b[len(t.b)-tailBufferSize:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return strings.TrimSpace(string(t.b)) }

// Progress is reported at most once a second and only when it moved by this much: the
// uploader's card polls every few seconds anyway, and every report is a gRPC call.
const (
	progressMinStep     = 2
	progressMinInterval = time.Second
)

// progressWriter turns ffmpeg's -progress output into percentages of the input's
// duration. Only the position matters: the lines come in blocks of key=value pairs.
type progressWriter struct {
	totalMicros float64
	report      func(percent int)
	now         func() time.Time

	partial []byte
	last    int
	lastAt  time.Time
}

// newProgressWriter reports to report. Without a duration there is nothing to measure
// against, so nothing is reported.
func newProgressWriter(durationSeconds float64, report func(percent int)) *progressWriter {
	return &progressWriter{totalMicros: durationSeconds * 1e6, report: report, now: time.Now, last: -progressMinStep}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.partial = append(w.partial, p...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}
		w.line(string(w.partial[:i]))
		w.partial = w.partial[i+1:]
	}
	return len(p), nil
}

func (w *progressWriter) line(line string) {
	// out_time_ms is microseconds as well, a long-standing ffmpeg quirk, so out_time_us
	// is the unambiguous one. It reads N/A until the first frame is out.
	v, ok := strings.CutPrefix(strings.TrimSpace(line), "out_time_us=")
	if !ok || w.totalMicros <= 0 {
		return
	}
	micros, err := strconv.ParseFloat(v, 64)
	if err != nil || micros < 0 {
		return
	}
	// 100 is for VODReadyNotification to say: the playlist is only there after the swap.
	percent := min(int(micros/w.totalMicros*100), 99)
	now := w.now()
	if percent < w.last+progressMinStep || now.Sub(w.lastAt) < progressMinInterval {
		return
	}
	w.last, w.lastAt = percent, now
	w.report(percent)
}

// progressReporter reports progress of one version of a stream's import to notify.
//
// Best effort, like everything about progress: it runs in the goroutine that drains
// ffmpeg's stdout, so waiting for a busy or gone receiver would stall ffmpeg itself. The
// next report replaces a dropped one.
func progressReporter(notify chan<- *protobuf.Notification, streamID uint64, streamVersion string) func(percent int) {
	return func(percent int) {
		n := &protobuf.Notification{
			Data: &protobuf.Notification_TranscodingProgress{
				TranscodingProgress: &protobuf.TranscodingProgressNotification{
					Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
					StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
					Progress:      ptr.Take(int32(percent)),
				},
			},
		}
		select {
		case notify <- n:
		default:
		}
	}
}
