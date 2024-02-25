package actions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

func (a *ActionProvider) TranscodeAction() *Action {
	return &Action{
		Type: TranscodeAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {

			files, ok := ctx.Value("files").([]string)
			if !ok {
				return ctx, ErrActionInputWrongType
			}
			if files == nil {
				log.Error("no files to transcode", "files", files)
				return ctx, ErrRequiredContextValNotFound
			}
			streamID, ok := ctx.Value("stream").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain stream", ErrRequiredContextValNotFound)
			}
			courseID, ok := ctx.Value("course").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain course", ErrRequiredContextValNotFound)
			}
			version, ok := ctx.Value("version").(string)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain version", ErrRequiredContextValNotFound)
			}

			log.Info("transcoding", "files", files)
			time.Sleep(time.Second)
			// parse output from previous streamAction
			fileName, ok := ctx.Value("files").([]string)
			if !ok {
				return ctx, ErrActionInputWrongType
			}
			filenames := ""
			if len(fileName) == 1 {
				filenames = fileName[0]
			} else {
				filenames = `"concat:` + fileName[0]
				for i := 1; i < len(fileName); i++ {
					filenames += "|" + fileName[i]
				}
				filenames += `"`
			}

			outputName := a.GetMassDir(courseID, streamID, version) + "/" + time.Now().Format("2006-01-02") + ".mp4"
			i := 1
			_, err := os.Stat(outputName)
			for err == nil {
				if errors.Is(err, os.ErrNotExist) {
					break
				}
				outputName = fmt.Sprintf(a.GetMassDir(courseID, streamID, version)+"/"+time.Now().Format("2006-01-02")+"_%d"+".mp4", i)
				_, err = os.Stat(outputName)
				i++
			}

			// Pass 1 of audio normalization.
			// Audio normalization is only applied, when only one video of the stream exists. Reasons for this:
			// 1. Multiple videos existing for one stream is typically caused by a shutdown of a runner. This does not happen frequently.
			// 2. It's much more inefficient to apply the audio normalization operation for more than one file:
			// 2.1 Instead of 2 passes, 3 passes are needed: concat - get parameter - execute;
			// 2.2 Video files need to be stored 3 times instead of twice (including the raw .ts files), at least temporarily
			//		(Extracting and only operating/storing the audio is unacceptable due to the problem mentioned in one comment of this answer: https://stackoverflow.com/a/27413824)
			var info *InfoForAudioNormalization = nil
			if len(fileName) == 1 {
				info, err = getInfoForAudioNormalization(ctx, a.Cmd.AudioNormalize1, fileName[0])
			}

			cmd := fmt.Sprintf(a.Cmd.Transcoding, filenames, outputName)
			// Pass 2 of audio normalization
			// Applied only when pass 1 is successfully executed
			// It does the same to the video, and additionally normalizes the audio with the given parameters from pass 1
			if info != nil {
				cmd = fmt.Sprintf(a.Cmd.AudioNormalize2, filenames,
					info.InputI, info.InputTp, info.InputLra, info.InputThresh, info.TargetOffset, outputName)
				log.Info("Transcoding with audio normalization", "files", files)
			}
			c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
			c.Stderr = os.Stderr
			err = c.Start()
			if err != nil {
				return ctx, err
			}
			err = c.Wait()
			return ctx, err
		},
	}
}

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

func getInfoForAudioNormalization(ctx context.Context, cmdFmt string, filename string) (*InfoForAudioNormalization, error) {
	// Errors during pass 1 won't propagate to outside.
	// But errors will prevent pass 2 from executing, ultimately resulting in the video not undergoing audio normalization.
	cmd := fmt.Sprintf(cmdFmt, filename)
	c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
	c.Stderr = os.Stderr
	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdoutPipe.Close()

	err = c.Start()
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	scanner := bufio.NewScanner(stdoutPipe)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() { // Reads the output from FFmpeg
		defer wg.Done()
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line + "\n")
		}
	}()

	err = c.Wait()
	if err != nil {
		return nil, err
	}

	wg.Wait()

	info := &InfoForAudioNormalization{}
	err = extractAndParseJSON(output.String(), info)
	if err != nil {
		return nil, err
	}
	return info, err
}

func extractAndParseJSON(output string, info *InfoForAudioNormalization) error {
	re := regexp.MustCompile(`(?s)\{.*}`) // Finds JSON data from the output
	matches := re.FindStringSubmatch(output)

	if len(matches) == 0 {
		return fmt.Errorf("no JSON data found")
	}

	jsonData := matches[0]
	err := json.Unmarshal([]byte(jsonData), info)
	if err != nil {
		return err
	}

	return nil
}
