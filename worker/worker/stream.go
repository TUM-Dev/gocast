package worker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/TUM-Dev/gocast/worker/cfg"
	log "github.com/sirupsen/logrus"
)

// stream records and streams a lecture hall to the lrz
func stream(streamCtx *StreamContext) {
	// add 10 minutes padding to stream end in case lecturers do lecturer things
	streamUntil := streamCtx.endTime.Add(time.Minute * 10)
	log.WithFields(log.Fields{"source": streamCtx.sourceUrl, "end": streamUntil, "fileName": streamCtx.getRecordingFileName()}).
		Info("streaming lecture hall")
	S.startStream(streamCtx)
	defer S.endStream(streamCtx)
	// in case ffmpeg dies retry until stream should be done.
	lastErr := time.Now().Add(time.Minute * -1)
	errCount := 0
	for time.Now().Before(streamUntil) && !streamCtx.stopped {
		timeout := fmt.Sprintf("%.0f", time.Until(streamUntil).Seconds())
		args := buildFFmpegArgs(streamCtx, timeout)
		cmd := exec.Command("ffmpeg", args...)
		recordFile, err := os.OpenFile(streamCtx.getRecordingFileName(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			cmd.Stdout = recordFile
		} else {
			log.WithError(err).Error("Could not open file for ffmpeg stdout")
		}

		// persist stream command in context, so it can be killed later
		streamCtx.streamCmd = cmd
		log.WithField("cmd", cmd.String()).Info("Starting stream")
		ffmpegErr, errFfmpegErrFile := os.OpenFile(fmt.Sprintf("%s/ffmpeg_%s.log", cfg.LogDir, streamCtx.getStreamName()), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if errFfmpegErrFile == nil {
			cmd.Stderr = ffmpegErr
		} else {
			log.WithError(errFfmpegErrFile).Error("Could not create file for ffmpeg stdErr")
		}
		// Create a new pgid for the new process, so we don't kill the parent process when ending the stream
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		err = cmd<.Run()
		if recordFile != nil {
			_ = recordFile.Close()
		}
		if ffmpegErr != nil {
			_ = ffmpegErr.Close()
		}

		if err != nil && !streamCtx.stopped {
			errCount++
			if errCount > 20 && strings.Contains(streamCtx.sourceUrl, "localhost") {
				// assume 20 seconds of inactivity by self - streamer as offline
				streamCtx.stopped = true
				return
			}
			errorWithBackoff(&lastErr, "Error while streaming (run)", err)
			if errFfmpegErrFile == nil {
				_ = ffmpegErr.Close()
			}
			continue
		}
		if errFfmpegErrFile == nil {
			_ = ffmpegErr.Close()
		}
	}
	streamCtx.streamCmd = nil
}

func buildFFmpegArgs(streamCtx *StreamContext, timeout string) []string {
	args := []string{
		"-hide_banner", "-nostats",
		"-t", timeout,
		"-i", streamCtx.sourceUrl,
		"-map", "0",
		"-c", "copy",
		"-f", "mpegts", "-",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-maxrate", "2500k",
		"-bufsize", "3000k",
		"-g", "60",
		"-r", "30",
		"-x264-params", "keyint=60:scenecut=0",
		"-c:a", "aac",
		"-ar", "44100",
		"-b:a", "128k",
		"-f", "flv",
		fmt.Sprintf("%s/%s", streamCtx.ingestServer, streamCtx.streamName),
	}

	if strings.Contains(streamCtx.sourceUrl, "rtsp") {
		args = append([]string{"-rtsp_transport", "tcp"}, args...)
	} else {
		args = append(args, "-rw_timeout", "5000000")
	}

	return args
}

// errorWithBackoff updates lastError and sleeps for a second if the last error was within this second
func errorWithBackoff(lastError *time.Time, msg string, err error) {
	log.WithFields(log.Fields{"lastErr": lastError}).WithError(err).Error(msg)
	if time.Now().Add(time.Second * -1).Before(*lastError) {
		log.Warn("too many errors, backing off a second.")
		time.Sleep(time.Second)
	}
	now := time.Now()
	*lastError = now
}
