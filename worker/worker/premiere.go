package worker

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/TUM-Dev/gocast/worker/cfg"
	log "github.com/sirupsen/logrus"
)

func streamPremiere(ctx *StreamContext) {
	// we're a little paranoid about our input as we can't control it:
	cmd := exec.Command(
		"ffmpeg", "-re", "-i", ctx.sourceUrl,
		"-pix_fmt", "yuv420p", "-vsync", "1", "-threads", "0", "-vcodec", "libx264",
		"-r", "30", "-g", "60", "-sc_threshold", "0",
		"-b:v", "2500k", "-bufsize", "3000k", "-maxrate", "3000k",
		"-preset", "veryfast", "-profile:v", "baseline", "-tune", "film",
		"-acodec", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000", "-af", "aresample=async=1:min_hard_comp=0.100000:first_pts=0",
		"-f", "flv", fmt.Sprintf("%s%s", ctx.ingestServer, ctx.streamName))
	log.WithField("cmd", cmd.String()).Info("Starting premiere")
	ffmpegErr, errFfmpegErrFile := os.OpenFile(fmt.Sprintf("%s/ffmpeg_%s.log", cfg.LogDir, ctx.getStreamName()), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if errFfmpegErrFile == nil {
		cmd.Stderr = ffmpegErr
	} else {
		log.WithError(errFfmpegErrFile).Error("Could not create file for ffmpeg stdErr")
	}
	err := cmd.Run()
	if err != nil {
		log.WithError(err).Error("Can't stream premiere")
	}
}
