package actions

import (
	"context"
	"fmt"
	"github.com/tum-dev/gocast/runner/protobuf"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//var edgeTemplate = "%s://%s/live/%s/%d-%s/playlist.m3u8" // e.g. "https://stream.domain.com/workerhostname123/1-COMB/playlist.m3u8"

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

			hostname, ok := ctx.Value("Hostname").(string)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain hostname", ErrRequiredContextValNotFound)
			}
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
			log.Info("streaming", "source", source, "end", time.Now().Second()+end.Second())

			//endingTime := time.Now().Add(time.Second * time.Duration(end.Second()))
			log.Info("streaming until", "end", end)

			streamAttempt := 0
			for time.Now().Before(end) && ctx.Err() == nil {
				streamAttempt++
				filename := filepath.Join(a.GetRecDir(courseID, streamID, version), fmt.Sprintf("%d.ts", streamAttempt))
				files = append(files, filename)
				livePlaylist := filepath.Join(a.GetLiveDir(courseID, streamID, version), end.Format("15-04-05"), "playlist.m3u8")
				_, err := os.Stat(a.GetLiveDir(courseID, streamID, version) + "/" + end.Format("15-04-05"))
				if err != nil {
					if os.IsNotExist(err) {
						err := os.Mkdir(a.GetLiveDir(courseID, streamID, version)+"/"+end.Format("15-04-05"), 0700)
						if err != nil {
							log.Warn("streamAction: stream folder couldn't be created", err)
							time.Sleep(5 * time.Second) // little backoff to prevent dossing source
							continue
						}
					}
				}

				src := ""
				if strings.HasPrefix(source, "rtsp") {
					src += "-rtsp_transport tcp"
				} else if strings.HasPrefix(source, "rtmp") {
					src += "-rw_timeout 5000000" // timeout selfstream	s after 5 seconds of no data
				} else {
					src += "-re" // read input at native framerate, e.g. when streaming a file in realtime
				}

				log.Info("streaming", "source", source, "end", time.Now().Second()+end.Second())

				//changing the end variable from a date to a duration and adding the duration to the current time
				cmd := fmt.Sprintf(a.Cmd.Stream, src, time.Until(end).Seconds(), source, filename, filepath.Join(a.GetLiveDir(courseID, streamID, version), end.Format("15-04-05")), livePlaylist)

				c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
				c.Stderr = os.Stderr
				log.Info("constructed stream command", "cmd", c.String())

				err = c.Start()
				if err != nil {
					log.Warn("streamAction: ", err)
					time.Sleep(5 * time.Second) // little backoff to prevent dossing source
					continue
				}
				resp := a.Server.NotifyStreamStarted(ctx, &protobuf.StreamStarted{
					Hostname: hostname,
					StreamID: uint32(streamID),
					CourseID: uint32(courseID),
					Version:  version,
					HLSUrl:   fmt.Sprintf("http://localhost:8187/%d/%d/%s/%s/playlist.m3u8", courseID, streamID, version, end.Format("15-04-05")),
				})
				if resp.Ok != true {
					log.Warn("streamAction: NotifyStreamStarted failed")
					time.Sleep(5 * time.Second) // little backoff to prevent dossing source
					continue
				}
				err = c.Wait()
				if err != nil {
					log.Warn("stream command exited", "err", err)
					time.Sleep(5 * time.Second) // little backoff to prevent dossing source
					continue
				}
				log.Info("stream finished. now sending notification")
				resp = a.Server.NotifyStreamEnded(ctx, &protobuf.StreamEnded{
					RunnerID: hostname,
					StreamID: uint32(streamID),
					CourseID: uint32(courseID),
				})
			}
			return set(ctx, "files", files), nil
		},
	}
}
