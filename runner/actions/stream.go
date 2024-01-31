package actions

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var edgeTemplate = "%s://%s/live/%s/%d-%s/playlist.m3u8" // e.g. "https://stream.domain.com/workerhostname123/1-COMB/playlist.m3u8"

// StreamAction streams a video. in is ignored. out is a []string containing the filenames of the recorded stream.
// ctx must contain the following values:
// - streamID (uint64) // e.g. 1
// - courseID (uint64) // e.g. 1
// - version (string) // e.g. "PRES", "CAM", "COMB"
// - source (string) // e.g. "rtmp://localhost:1935/live/abc123" for selfstreams or "rtsp://1.2.3.4/extron1" for auditoriums
// - end (time.Time) // the end of the stream for auditoriums or an end date far in the future for selfstreams.
// after StreamAction is done, the following values are set in ctx:
// - files ([]string) // a list of files that were created during the stream
func (a *ActionProvider) StreamAction() *Action {
	return &Action{
		Type: StreamAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			// files will contain all files that were created during the stream
			var files []string

			streamID, ok := ctx.Value("stream").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain stream", ErrRequiredContextValNotFound)
			}
			courseID, ok := ctx.Value("course").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain courseID", ErrRequiredContextValNotFound)
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
			log.Info("streaming", "source", source, "end", end)

			streamAttempt := 0
			for time.Now().Before(end) && ctx.Err() == nil {
				streamAttempt++
				filename := filepath.Join(a.GetRecDir(courseID, streamID, version), fmt.Sprintf("%d.ts", streamAttempt))
				files = append(files, filename)
				livePlaylist := filepath.Join(a.GetLiveDir(courseID, streamID, version), "playlist.m3u8")

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
				cmd += " -hls_time 2 -hls_list_size 3600 -hls_playlist_type event -hls_flags append_list -hls_segment_filename " + filepath.Join(a.GetLiveDir(courseID, streamID, version), "/%05d.ts")
				cmd += " " + livePlaylist

				c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
				c.Stderr = os.Stderr
				log.Info("constructed stream command", "cmd", c.String())
				err := c.Start()
				if err != nil {
					log.Warn("streamAction: ", err)
					time.Sleep(5 * time.Second) // little backoff to prevent dossing source
					continue
				}
				err = c.Wait()
				if err != nil {
					log.Warn("stream command exited", "err", err)
					time.Sleep(5 * time.Second) // little backoff to prevent dossing source
					continue
				}
			}
			return set(ctx, "files", files), nil
		},
	}
}
