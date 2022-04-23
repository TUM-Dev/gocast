package worker

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
)

func transcode(streamCtx *StreamContext) error {
	err := prepare(streamCtx.getTranscodingFileName())
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	// create command fitting its content with appropriate niceness:
	in := streamCtx.getRecordingFileName()
	out := streamCtx.getTranscodingFileName()
	switch streamCtx.streamVersion {
	case "CAM":
		// compress camera image slightly more
		cmd = exec.Command("nice", "-n", "10", "ffmpeg", "-y", "-nostats", "-i", in, "-vsync", "2", "-c:v", "libx264", "-level", "4.0", "-movflags", "+faststart", "-c:a", "aac", "-b:a", "128k", "-crf", "26", out)
	case "PRES":
		cmd = exec.Command("nice", "-n", "9", "ffmpeg", "-y", "-nostats", "-i", in, "-vsync", "2", "-c:v", "libx264", "-level", "4.0", "-movflags", "+faststart", "-tune", "stillimage", "-c:a", "aac", "-b:a", "128k", "-crf", "20", out)
	case "COMB":
		cmd = exec.Command("nice", "-n", "8", "ffmpeg", "-y", "-nostats", "-i", in, "-vsync", "2", "-c:v", "libx264", "-level", "4.0", "-movflags", "+faststart", "-c:a", "aac", "-b:a", "128k", "-crf", "24", out)
	default:
		//unknown source, use higher compression and less priority
		cmd = exec.Command("nice", "-n", "10", "ffmpeg", "-y", "-nostats", "-i", in, "-vsync", "2", "-c:v", "libx264", "-level", "4.0", "-movflags", "+faststart", "-c:a", "aac", "-b:a", "128k", "-crf", "26", out)
	}
	log.WithFields(log.Fields{"input": in, "output": out, "command": cmd.String()}).Info("Transcoding")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"output": string(output)}).Error("Transcoding failed")
		return fmt.Errorf("transcode stream: %v", err)
	} else {
		log.WithField("stream", streamCtx.getStreamName()).Info("Transcoding finished")
	}
	log.Info("Start Probing duration")
	duration, err := getDuration(streamCtx.getTranscodingFileName())
	if err != nil {
		return fmt.Errorf("probe duration: %v", err)
	} else {
		streamCtx.duration = uint32(duration)
		log.WithField("duration", duration).Info("Probing duration finished")
	}
	return nil
}

// creates folder for output file if it doesn't exist
func prepare(out string) error {
	dir := filepath.Dir(out)
	err := os.MkdirAll(dir, 0750)
	if err != nil {
		return fmt.Errorf("create output directory for transcoding: %s", err)
	}
	return nil
}

// markForDeletion moves the file to $recfolder/.trash/
func markForDeletion(ctx *StreamContext) error {
	trashName := ctx.getRecordingTrashName()
	err := os.MkdirAll(filepath.Dir(trashName), 0750)
	if err != nil {
		return fmt.Errorf("create trash directory: %s", err)
	}
	err = os.Rename(ctx.getRecordingFileName(), ctx.getRecordingTrashName())
	if err != nil {
		return fmt.Errorf("move file to .trash: %s", err)
	}
	return persisted.AddDeletable(trashName)
}
