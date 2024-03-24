package actions

import (
	"context"
	"fmt"
	"github.com/tum-dev/gocast/runner/protobuf"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

func (a *ActionProvider) SilenceDetectAction() *Action {
	return &Action{
		Type: SilenceDetectAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {
			streamID, ok := ctx.Value("stream").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain stream", ErrRequiredContextValNotFound)
			}

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

			starts, ends := postprocess(silences)
			log.Info("Silences detected", "file", filename, "silences", silences)
			a.Server.NotifySilenceResults(ctx, &protobuf.SilenceResults{
				RunnerID: "0", // TODO: replace with runner ID
				StreamID: uint32(streamID),
				Starts:   starts,
				Ends:     ends,
			})

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
				Start: uint32(start),
				End:   0,
			})
		} else if strings.Contains(line, "silence_end:") {
			end, err := strconv.ParseFloat(strings.Split(strings.Split(line, "silence_end: ")[1], " |")[0], 32)
			if err != nil || silences == nil || len(silences) == 0 {
				return nil, err
			}
			silences[len(silences)-1].End = uint32(end)
		}
	}
	return silences, nil
}

// postprocess merges short durations of silence into units of silence,
// and returns starts and ends as two separate arrays
func postprocess(silences []silence) ([]uint32, []uint32) {
	if len(silences) >= 2 {
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
		silences = newSilences
	}

	var starts []uint32
	var ends []uint32
	for i := 1; i < len(silences); i++ {
		starts = append(starts, silences[i].Start)
		ends = append(ends, silences[i].End)
	}

	return starts, ends
}

type silence struct {
	Start uint32
	End   uint32
}
