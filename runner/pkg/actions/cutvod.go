package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const (
	reasonNothingLeft = "The cuts leave nothing of the video."
	// minKeepSeconds is the shortest stretch worth keeping. A sliver of a few frames is what
	// two cuts that nearly touch leave behind, and it only makes the video stutter.
	minKeepSeconds = 0.1
)

// TimeRange is a stretch of a video, in seconds from its start.
type TimeRange struct{ Start, End float64 }

// CutVOD re-encodes the version at the playlist d["sourceURL"] without the stretches in
// d["discard"]. It reads it from there, as the edge serves it, and writes the result to its own
// storage as the VOD of stream d["streamID"]: the source's own, which replaces it in place, or
// that of a copy.
//
// It is frame accurate, and so not a copy of the streams: cutting those would snap to the next
// keyframe, which are 20 seconds apart.
//
// Like ImportVOD, every failure is aborting and reported to the lecture's admin by a
// VODFailedNotification, and the previous VOD stays in place.
func CutVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	err := cutVOD(ctx, logger, notify, d, metrics)
	if err == nil {
		return nil
	}
	streamID, ok1 := d["streamID"].(uint64)
	streamVersion, ok2 := d["streamVersion"].(string)
	if ok1 && ok2 {
		reason := reasonOther
		var ie *importError
		switch {
		case ctx.Err() != nil:
			reason = "The cut was interrupted. Please try again."
		case errors.As(err, &ie):
			reason = ie.reason
		}
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

func cutVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	input, _ := d["sourceURL"].(string)
	if input == "" {
		return AbortingError(fmt.Errorf("no source url in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	discard, _ := d["discard"].([]TimeRange)

	metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Inc()
	defer metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Dec()

	probe, err := ffmpeg.Probe(ctx, input)
	if err != nil {
		return failImport(reasonUnreadable, fmt.Errorf("probe source: %w", err))
	}
	if probe.Codec("video") == "" {
		return failImport(reasonNoVideo, fmt.Errorf("source has no video stream"))
	}
	keep := keepRanges(discard, probe.Duration())
	if len(keep) == 0 {
		return failImport(reasonNothingLeft, fmt.Errorf("no range left of %.1fs after discarding %v", probe.Duration(), discard))
	}

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	if err := os.MkdirAll(path.Dir(vodDir), os.ModePerm); err != nil {
		return failImport(reasonStorage, fmt.Errorf("create stream directory: %w", err))
	}
	// Next to the final directory and swapped in at the end, see importVOD.
	workDir, err := os.MkdirTemp(path.Dir(vodDir), ".cutting-")
	if err != nil {
		return failImport(reasonStorage, fmt.Errorf("create work directory: %w", err))
	}
	defer func() { _ = os.RemoveAll(workDir) }() // no-op once renamed

	logger.Info("cutting VOD", "keep", keep)
	command := exec.CommandContext(ctx, "ffmpeg", cutArgs(input, workDir, keep, probe.Codec("audio") != "")...)
	command.WaitDelay = 10 * time.Second
	var stderr tailBuffer
	command.Stderr = &stderr
	// Progress is the position in the result, so it is measured against its length.
	command.Stdout = newProgressWriter(totalLength(keep), progressReporter(notify, streamID, streamVersion))
	if err := command.Run(); err != nil {
		return failImport(reasonConversion, fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String()))
	}

	if err := replaceVOD(workDir, vodDir); err != nil {
		return failImport(reasonStorage, err)
	}

	// The actions after this one read the VOD from here.
	d["playlistURL"] = path.Join(vodDir, "playlist.m3u8")

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

// keepRanges inverts the discarded stretches of a video of the given length into the ones to
// keep. Discards may be unsorted or overlap, and reach beyond the video.
func keepRanges(discard []TimeRange, duration float64) []TimeRange {
	sorted := slices.Clone(discard)
	slices.SortFunc(sorted, func(a, b TimeRange) int {
		switch {
		case a.Start < b.Start:
			return -1
		case a.Start > b.Start:
			return 1
		}
		return 0
	})

	var keep []TimeRange
	pos := 0.0
	add := func(start, end float64) {
		if end-start >= minKeepSeconds {
			keep = append(keep, TimeRange{start, end})
		}
	}
	for _, r := range sorted {
		if r.Start > pos {
			add(pos, min(r.Start, duration))
		}
		pos = max(pos, r.End)
		if pos >= duration {
			return keep
		}
	}
	add(pos, duration)
	return keep
}

func totalLength(ranges []TimeRange) float64 {
	var sum float64
	for _, r := range ranges {
		sum += r.End - r.Start
	}
	return sum
}

// cutArgs builds the ffmpeg arguments that write the given ranges of input, one after the
// other, as a VOD playlist into vodDir.
func cutArgs(input, vodDir string, keep []TimeRange, hasAudio bool) []string {
	args := []string{
		"-nostdin", "-y", "-nostats", "-progress", "pipe:1", "-i", input,
		"-filter_complex", cutFilter(keep, hasAudio),
		"-map", "[v]",
	}
	if hasAudio {
		args = append(args, "-map", "[a]")
	}
	args = append(args,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
		"-force_key_frames", "expr:gte(t,n_forced*20)",
	)
	if hasAudio {
		args = append(args, "-c:a", "aac")
	}
	return append(args, hlsOutputArgs(vodDir)...)
}

// cutFilter is the filter graph that joins the ranges. Each piece keeps its timestamps from the
// source through trim, so they are moved back to start at 0 for concat to place them one after
// the other.
func cutFilter(keep []TimeRange, hasAudio bool) string {
	var graph, joined strings.Builder
	for i, r := range keep {
		start, end := seconds(r.Start), seconds(r.End)
		fmt.Fprintf(&graph, "[0:v]trim=start=%s:end=%s,setpts=PTS-STARTPTS[v%d];", start, end, i)
		fmt.Fprintf(&joined, "[v%d]", i)
		if hasAudio {
			fmt.Fprintf(&graph, "[0:a]atrim=start=%s:end=%s,asetpts=PTS-STARTPTS[a%d];", start, end, i)
			fmt.Fprintf(&joined, "[a%d]", i)
		}
	}
	audio := 0
	if hasAudio {
		audio = 1
	}
	fmt.Fprintf(&graph, "%sconcat=n=%d:v=1:a=%d[vc]", joined.String(), len(keep), audio)
	if hasAudio {
		graph.WriteString("[a]")
	}
	// Even dimensions, as yuv420p requires them.
	graph.WriteString(";[vc]scale=trunc(iw/2)*2:trunc(ih/2)*2[v]")
	return graph.String()
}

func seconds(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }
