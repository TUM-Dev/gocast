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
					ImportId:      importID(d),
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
	// A browser recording's segments, of which uploadPath is the first, and how long
	// before each the recording of this version was interrupted.
	inputs := []string{uploadPath}
	if paths, ok := d["uploadPaths"].([]string); ok && len(paths) > 1 {
		inputs = paths
	}
	gaps, _ := d["uploadGaps"].([]float64)
	trailing, _ := d["uploadTrailingGap"].(float64)
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
	var duration float64
	if len(inputs) == 1 && !hasGap(gaps) && trailing <= 0 {
		probe := probes[0]
		transcode := needsTranscode(probe.Codec("video"), probe.Codec("audio"), probe.Level(), probe.PixelFormat())
		logger.Info("importing upload", "transcode", transcode, "file", uploadPath)
		args = importArgs(uploadPath, workDir, transcode)
		duration = probe.Duration()
		if duration <= 0 {
			// Only for the progress: a MediaRecorder's WebM has no duration in its header.
			duration, _ = ffmpeg.MeasureDuration(ctx, uploadPath)
		}
	} else {
		segments := make([]segment, len(inputs))
		for i, file := range inputs {
			segments[i] = segment{path: file, audio: probes[i].Codec("audio") != ""}
			if i < len(gaps) {
				segments[i].gapBefore = gaps[i]
			}
			// The length is what silence for a segment without audio has to match.
			d, err := ffmpeg.MeasureDuration(ctx, file)
			if err != nil {
				return failImport(reasonUnreadable, fmt.Errorf("measure segment %d: %w", i, err))
			}
			segments[i].duration = d
			duration += segments[i].gapBefore + d
		}
		duration += trailing
		width, height := probes[0].Dimensions()
		logger.Info("joining uploaded segments", "segments", len(inputs), "width", width, "height", height, "gaps", gaps, "trailing", trailing)
		args = concatArgs(segments, trailing, workDir, width, height)
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
	command.Stdout = newProgressWriter(duration, progressReporter(notify, streamID, streamVersion))
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
				ImportId:      importID(d),
			},
		},
	}
	return nil
}

// importID is the import's ID for its notifications, nil for none: see
// PrepareUploadRequest.import_id.
func importID(d map[string]any) *string {
	if id, ok := d["importID"].(string); ok && id != "" {
		return &id
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

// segment is one file of a browser recording.
type segment struct {
	path string
	// audio is whether the file has an audio stream; one without gets silence.
	audio bool
	// duration is how long its video runs, in seconds.
	duration float64
	// gapBefore is how long, in seconds, the recording of this version was interrupted
	// before the segment, while the other version went on: filled with black and
	// silence, so both versions keep one timeline, as split view plays them.
	gapBefore float64
}

func hasGap(gaps []float64) bool {
	for _, g := range gaps {
		if g > 0 {
			return true
		}
	}
	return false
}

// concatArgs builds the ffmpeg arguments that join the segments of a browser recording,
// in order, into one VOD playlist in vodDir. Always transcoded: the timestamps of each
// segment start at zero, and segments can differ in size, as a screen shared anew does.
// The concat filter wants the same streams in each part and the same size, so every
// part gets video and audio, fitted into the first segment's size: a gap is a black
// picture with silence, a segment without audio gets silence of its length. trailing is
// a gap after the last segment: the time this version missed at the end.
func concatArgs(segments []segment, trailing float64, vodDir string, width, height int) []string {
	if width <= 0 || height <= 0 {
		width, height = 1920, 1080
	}
	// Even dimensions, as yuv420p requires them.
	width, height = width/2*2, height/2*2

	args := []string{"-nostdin", "-y", "-nostats", "-progress", "pipe:1"}
	next := 0
	input := func(a ...string) int {
		args = append(args, a...)
		next++
		return next - 1
	}
	silence := func(seconds float64) int {
		return input("-f", "lavfi", "-t", formatSeconds(seconds), "-i", "anullsrc=r=48000:cl=stereo")
	}

	type part struct{ video, audio string }
	var parts []part
	gap := func(seconds float64) {
		black := input("-f", "lavfi", "-t", formatSeconds(seconds), "-i",
			fmt.Sprintf("color=c=black:s=%dx%d:r=30", width, height))
		parts = append(parts, part{fmt.Sprintf("%d:v", black), fmt.Sprintf("%d:a", silence(seconds))})
	}
	for _, seg := range segments {
		if seg.gapBefore > 0 {
			gap(seg.gapBefore)
		}
		file := input("-i", seg.path)
		audio := fmt.Sprintf("%d:a", file)
		if !seg.audio {
			audio = fmt.Sprintf("%d:a", silence(seg.duration))
		}
		parts = append(parts, part{fmt.Sprintf("%d:v", file), audio})
	}
	if trailing > 0 {
		gap(trailing)
	}

	var filter, joined strings.Builder
	for i, p := range parts {
		fmt.Fprintf(&filter, "[%s]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p[v%d];",
			p.video, width, height, width, height, i)
		fmt.Fprintf(&filter, "[%s]aresample=48000,aformat=channel_layouts=stereo[a%d];", p.audio, i)
		fmt.Fprintf(&joined, "[v%d][a%d]", i, i)
	}
	fmt.Fprintf(&filter, "%sconcat=n=%d:v=1:a=1[v][a]", joined.String(), len(parts))

	args = append(args,
		"-filter_complex", filter.String(), "-map", "[v]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-c:a", "aac",
		// Keyframes on the segment boundaries keep the segments close to hls_time.
		"-force_key_frames", "expr:gte(t,n_forced*20)",
	)
	return append(args, hlsOutputArgs(vodDir)...)
}

func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
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
