package actions

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

func (a *ActionProvider) SilenceDetectAction() *Action {
	return &Action{
		Type: SilenceDetectAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			filename, ok := ctx.Value("outputFilename").(string)
			if !ok {
				errMsg := "no transcoded file to process for silence detection"
				log.Error(errMsg)
				return ctx, fmt.Errorf(errMsg)
			}

			log.Info("Start detecting silence", "file", filename)
			cmd := fmt.Sprintf(a.Cmd.SilenceDetect, filename)
			c := exec.CommandContext(ctx, "ffmpeg", strings.Split(cmd, " ")...)
			output, err := c.CombinedOutput()
			if err != nil {
				log.Error("Error executing command", "error", err)
				return ctx, err
			}

			silences, err := parseSilence(string(output))
			if err != nil {
				log.Error("Error parsing silence", "error", err)
				return ctx, err
			}

			silences = postprocess(silences)
			// TODO: send the results to TUM-Live server, instead of just logging them
			log.Info("Silences detected", "file", filename, "silences", silences)

			return ctx, nil
		},
	}
}

func parseSilence(output string) ([]silence, error) {
	var silences []silence
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "silence_start:") {
			start, err := strconv.ParseFloat(strings.Split(line, "silence_start: ")[1], 32)
			if err != nil {
				return nil, err
			}
			silences = append(silences, silence{
				Start: uint(start),
				End:   0,
			})
		} else if strings.Contains(line, "silence_end:") {
			end, err := strconv.ParseFloat(strings.Split(strings.Split(line, "silence_end: ")[1], " |")[0], 32)
			if err != nil || silences == nil || len(silences) == 0 {
				return nil, err
			}
			silences[len(silences)-1].End = uint(end)
		}
	}
	return silences, nil
}

// postprocess merges short durations of silence into units of silence
func postprocess(silences []silence) []silence {
	if len(silences) < 2 {
		return silences
	}
	if silences[0].Start < 30 {
		silences[0].Start = 0
	}
	var newSilences []silence
	newSilences = append(newSilences, silences[0])
	for i := 1; i < len(silences); i++ {
		if silences[i].Start-newSilences[len(newSilences)-1].End < 30 {
			newSilences[len(newSilences)-1].End = silences[i].End
		} else {
			newSilences = append(newSilences, silences[i])
		}
	}
	return newSilences
}

type silence struct {
	Start uint
	End   uint
}
