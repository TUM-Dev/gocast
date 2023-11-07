package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/TUM-Dev/gocast/worker/cfg"
	"github.com/TUM-Dev/gocast/worker/pb"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type InfoForAudioNormalization struct {
	InputI            string `json:"input_i"`
	InputTp           string `json:"input_tp"`
	InputLra          string `json:"input_lra"`
	InputThresh       string `json:"input_thresh"`
	OutputI           string `json:"output_i"`
	OutputTp          string `json:"output_tp"`
	OutputLra         string `json:"output_lra"`
	OutputThresh      string `json:"output_thresh"`
	NormalizationType string `json:"normalization_type"`
	TargetOffset      string `json:"target_offset"`
}

const (
	// The selection of loudnorm parameters follows the EBU R128 standard
	loudnormConfig = "I=-23:TP=-2:LRA=7"
)

func getInfoForAudioNormalization(niceness int, mediaFilename string) (*InfoForAudioNormalization, error) {
	c := []string{
		"-n", fmt.Sprintf("%d", niceness),
		"ffmpeg", "-nostats", "-y",
		"-i", mediaFilename,
		"-af", "loudnorm=" + loudnormConfig + ":print_format=json",
		"-f", "null", "-"}
	cmd := exec.Command("nice", c...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"input": mediaFilename, "command": cmd.String()}).
			Warn("Failed to get information for audio normalization")
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	jsonStr := "{"
	shouldAppendToStrOfJson := false
	for _, str := range lines {
		if shouldAppendToStrOfJson {
			jsonStr += str
		}
		if str == "{" {
			shouldAppendToStrOfJson = true
		} else if str == "}" {
			shouldAppendToStrOfJson = false
		}
	}
	jsonData := []byte(jsonStr)

	var info InfoForAudioNormalization

	err = json.Unmarshal(jsonData, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func buildCommand(niceness int, infile string, outfile string, tune string, crf int, audioNormInfo *InfoForAudioNormalization) *exec.Cmd {
	c := []string{
		"-n", fmt.Sprintf("%d", niceness),
		"ffmpeg", "-nostats", "-loglevel", "error", "-y",
		"-progress", "-",
		"-i", infile,
		"-vsync", "2", "-c:v", "libx264", "-level", "4.0", "-movflags", "+faststart"}
	if tune != "" {
		c = append(c, "-tune", tune)
	}
	if audioNormInfo != nil {
		c = append(c, "-af", fmt.Sprintf(
			"loudnorm="+loudnormConfig+":measured_i=%s:measured_tp=%s:measured_lra=%s:measured_thresh=%s:offset=%s:linear=true",
			audioNormInfo.InputI, audioNormInfo.InputTp, audioNormInfo.InputLra, audioNormInfo.InputThresh, audioNormInfo.TargetOffset))
	}
	c = append(c, "-c:a", "aac", "-b:a", "128k", "-crf", fmt.Sprintf("%d", crf), outfile)
	return exec.Command("nice", c...)
}

func transcode(streamCtx *StreamContext) error {
	log.Info("transcoding")

	progressChan := make(chan int32, 1)
	go func() {
		errs := 0
		for errs < 100 { // retry in case of timeouts or TUM-Live unavailability.
			err := reportProgress(streamCtx, progressChan)
			if err != nil {
				errs++
				time.Sleep(time.Second * 5) // backoff
			} else {
				return
			}
			if err != io.EOF {
				log.Warn(err)
			}
		}
	}()
	// Make sure reportProgress can exit with 100% when function exits
	defer func() { progressChan <- 100 }()

	err := prepare(streamCtx.getTranscodingFileName())
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	// create command fitting its content with appropriate niceness:
	in := streamCtx.getRecordingFileName()
	inputTime, err := getDuration(in)
	if err != nil {
		inputTime = 1
	}

	out := streamCtx.getTranscodingFileName()

	audioNormInfo, err := getInfoForAudioNormalization(10, in)

	switch streamCtx.streamVersion {
	case "CAM":
		// compress camera image slightly more
		cmd = buildCommand(10, in, out, "", 26, audioNormInfo)
	case "PRES":
		cmd = buildCommand(9, in, out, "stillimage", 20, audioNormInfo)
	case "COMB":
		cmd = buildCommand(8, in, out, "", 24, audioNormInfo)
	default:
		//unknown source, use higher compression and less priority
		cmd = buildCommand(10, in, out, "", 26, audioNormInfo)
	}
	log.WithFields(log.Fields{"input": in, "output": out, "command": cmd.String()}).Info("Transcoding")
	streamCtx.transcodingCmd = cmd
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Warn(err)
		return err
	}
	if err := cmd.Start(); err != nil {
		log.Warn(err)
		return err
	}

	// send progress to tumlive on stderr output:
	output := handleTranscodingOutput(stderr, inputTime, progressChan)

	err = cmd.Wait()
	if err != nil {
		log.WithFields(log.Fields{"output": output}).Error("Transcoding failed")
		return fmt.Errorf("transcode stream: %w", fmt.Errorf("%w: %s", err, output))
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

func handleTranscodingOutput(stderr io.ReadCloser, inputTime float64, progressChan chan int32) string {
	output := ""
	lastSend := -1
	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		m := scanner.Text()
		lines := strings.Split(m, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "time=") {
				// format: time=HH:MM:SS.MICROSECONDS
				tstr := strings.Split(strings.TrimSpace(line), "=")
				if len(tstr) == 2 {
					parsed, err := time.Parse("15:04:05", strings.Split(tstr[1], ".")[0])
					if err != nil {
						log.Info(err)
						continue
					}
					progress := int((float64(parsed.Hour()*60*60+parsed.Minute()*60+parsed.Second()) / inputTime) * 100)
					if progress > lastSend {
						progressChan <- int32(progress)
						lastSend = progress
					}
				}
			} else {
				output += line + " "
			}
		}
	}
	return output
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

func reportProgress(stream *StreamContext, p chan int32) error {
	client, conn, err := GetClient()
	if err != nil {
		log.WithError(err).Error("Unable to dial tumlive")
		return err
	}
	defer closeConnection(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	str, err := client.NotifyTranscodingProgress(ctx)
	if err != nil {
		return err
	}
	for {
		curP := <-p
		if curP == 100 {
			return nil
		}
		err = str.Send(&pb.NotifyTranscodingProgressRequest{
			WorkerID: cfg.WorkerID,
			StreamId: stream.streamId,
			Version:  stream.streamVersion,
			Progress: curP,
		})
		if err != nil {
			return err
		}
	}
}

func transcodeAudio(ctx *StreamContext) error {
	S.startTranscodingAudio(ctx.getStreamName())
	defer S.endTranscodingAudio(ctx.getStreamName())

	input := ctx.getTranscodingFileName()
	output := ctx.getAudioTranscodingFileName()
	cmd := exec.Command("ffmpeg",
		"-y",
		"-v", "quiet",
		"-i", input,
		"-c:a", "aac",
		"-vn", output)
	log.WithFields(log.Fields{"input": input, "output": output, "command": cmd.String()}).Info("Transcoding audio")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("transcode stream audio: %w", fmt.Errorf("%w: %s", err, out))
	}

	return nil
}
