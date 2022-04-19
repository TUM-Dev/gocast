package worker

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
	"time"
)

//record records a source until endTime +10 minutes without pushing to lrz
func record(streamCtx *StreamContext) {
	// add 10 minutes padding to stream end in case lecturers do lecturer things
	recordUntil := streamCtx.endTime.Add(time.Minute * 10)
	log.WithFields(log.Fields{"source": streamCtx.sourceUrl, "end": recordUntil, "fileName": streamCtx.getRecordingFileName()}).
		Info("Recording lecture hall")

	// in case ffmpeg dies retry until stream should be done.
	for time.Now().Before(recordUntil) {
		// recordings are made raw without further encoding
		var cmd *exec.Cmd
		if strings.Contains(streamCtx.sourceUrl, "rtsp") {
			cmd = exec.Command(
				"ffmpeg", "-nostats", "-rtsp_transport", "tcp",
				"-t", fmt.Sprintf("%.0f", time.Until(recordUntil).Seconds()), // timeout ffmpeg when stream is finished
				"-i", streamCtx.sourceUrl,
				"-map", "0",
				"-c:v", "copy",
				"-c:a", "copy",
				"-f", "mpegts", "-")
		} else {
			cmd = exec.Command(
				"ffmpeg", "-nostats",
				"-t", fmt.Sprintf("%.0f", time.Until(recordUntil).Seconds()), // timeout ffmpeg when stream is finished
				"-i", streamCtx.sourceUrl,
				"-map", "0",
				"-c:v", "copy",
				"-c:a", "copy",
				"-f", "mpegts", "-")
		}
		outfile, err := os.OpenFile(streamCtx.getRecordingFileName(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.WithError(err).Error("Unable to create file for recording")
			time.Sleep(time.Second) // sleep a second to prevent high load
			continue
		}
		cmd.Stdout = outfile
		err = cmd.Wait()
		if err != nil {
			log.WithError(err).Error("Error while recording")
			time.Sleep(time.Second) // prevent spamming logfiles and smp
			continue
		}
	}
}
