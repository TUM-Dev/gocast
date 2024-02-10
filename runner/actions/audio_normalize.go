package actions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
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

func (a *ActionProvider) AudioNormalizeAction() *Action {
	return &Action{
		Type: AudioNormalizeAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {

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

			fileName := fmt.Sprintf("%s/%s/%s/%s.mp4", a.MassDir, courseID, streamID, version)

			// Pass 1
			// Errors during pass 1 should not propagate to outside.
			// Thus, in the following code whenever an error occurs, ctx and nil are returned.
			// But errors will prevent pass 2 from executing, ultimately resulting in the video not undergoing audio normalization.
			cmd := fmt.Sprintf(a.Cmd.AudioNormalize1, fileName)
			c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
			c.Stderr = os.Stderr
			stdoutPipe, err := c.StdoutPipe()
			if err != nil {
				return ctx, nil
			}
			err = c.Start()
			if err != nil {
				return ctx, nil
			}

			var output bytes.Buffer
			scanner := bufio.NewScanner(stdoutPipe)
			go func() { // Reads the output from FFmpeg
				for scanner.Scan() {
					line := scanner.Text()
					output.WriteString(line + "\n")
				}
			}()

			err = c.Wait()
			if err != nil {
				return ctx, nil
			}

			info := &InfoForAudioNormalization{}
			err = extractAndParseJSON(output.String(), info)
			if err != nil {
				return ctx, nil
			}

			// pass 2
			// TODO
			return ctx, nil
		},
	}
}

func extractAndParseJSON(output string, info *InfoForAudioNormalization) error {
	re := regexp.MustCompile(`(?s)\{.*\}`) // Finds JSON data from the output
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
