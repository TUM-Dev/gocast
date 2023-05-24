package actions

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var edgeTemplate = "%s://%s/live/%s/%d-%s/playlist.m3u8" // e.g. "https://stream.domain.com/workerhostname123/1-COMB/playlist.m3u8"

// StreamAction streams a video. in is ignored. out is a []string containing the filenames of the recorded stream.
// ctx must contain the following values:
// - streamID (uint) // e.g. 1
// - version (string) // e.g. "PRES", "CAM", "COMB"
// - source (string) // e.g. "rtmp://localhost:1935/live/abc123" for selfstreams or "rtsp://1.2.3.4/extron1" for auditoriums
// - end (time.Time) // the end of the stream for auditoriums or an end date far in the future for selfstreams.
// after StreamAction is done, the following values are set in ctx:
// - files ([]string) // a list of files that were created during the stream
func StreamAction(ctx context.Context) (context.Context, error) {
	// files will contain all files that were created during the stream
	var files []string

	streamID, ok := ctx.Value("streamID").(uint)
	if !ok {
		return ctx, fmt.Errorf("%w: context doesn't contain streamID", ErrRequiredContextValNotFound)
	}
	version, ok := ctx.Value("version").(string)
	if !ok {
		return ctx, fmt.Errorf("%w: context doesn't contain version", ErrRequiredContextValNotFound)
	}
	source, ok := ctx.Value("source").(string)
	if !ok {
		return ctx, fmt.Errorf("%w: context doesn't contain source", ErrRequiredContextValNotFound)
	}
	end, ok := ctx.Value("end").(time.Time)
	if !ok {
		return ctx, fmt.Errorf("%w: context doesn't contain end", ErrRequiredContextValNotFound)
	}

	streamAttempt := 0
	for time.Now().Before(end) && context.Canceled == nil {
		filename := filepath.Join(recDir, fmt.Sprintf("%d-%s", streamID, version), fmt.Sprintf("%d.ts", streamAttempt))
		err := os.MkdirAll(filepath.Dir(filename), 0755)
		if err != nil {
			return ctx, fmt.Errorf("create recording directory: %w", err)
		}
		files = append(files, filename)
		livePlaylist := filepath.Join(liveSegmentDir, fmt.Sprintf("%d-%s/", streamID, version), "playlist.m3u8")
		err = os.MkdirAll(filepath.Dir(livePlaylist), 0755)
		if err != nil {
			return ctx, fmt.Errorf("create live playlist directory: %w", err)
		}

		cmd := "-y -hide_banner -nostats"
		if strings.HasPrefix(source, "rtsp") {
			cmd += " -rtsp_transport tcp"
		} else if strings.HasPrefix(source, "rtmp") {
			cmd += " -rw_timeout 5000000" // timeout selfstream	s after 5 seconds of no data
		} else {
			cmd += " -re" // read input at native framerate, e.g. when streaming a file in realtime
		}

		cmd += fmt.Sprintf(" -t %.0f", time.Until(end).Seconds())
		cmd += fmt.Sprintf(" -i %s", source)
		cmd += " -c:v copy -c:a copy -f mpegts " + filename // write original stream to file for later processing
		cmd += " -c:v libx264 -preset veryfast -tune zerolatency -maxrate 2500k -bufsize 3000k -g 60 -r 30 -x264-params keyint=60:scenecut=0 -c:a aac -ar 44100 -b:a 128k -f hls"
		// todo optional stream target
		cmd += " -hls_time 2 -hls_list_size 3600 -hls_flags append_list -hls_segment_filename " + filepath.Join(liveSegmentDir, fmt.Sprintf("%d-%s", streamID, version), "/%d.ts")
		cmd += " " + livePlaylist

		c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
		c.Stderr = log.StandardLogger().WriterLevel(log.InfoLevel)
		fmt.Println(c.String())
		err = c.Start()
		if err != nil {
			log.Warn("streamAction: ", err)
			time.Sleep(5 * time.Second) // little backoff to prevent dossing source
			continue
		}
		err = c.Wait()
		if err != nil {
			log.Warn("streamAction: ", err)
			time.Sleep(5 * time.Second) // little backoff to prevent dossing source
			continue
		}
		streamAttempt++
	}

	return set(ctx, "files", files), nil
}
