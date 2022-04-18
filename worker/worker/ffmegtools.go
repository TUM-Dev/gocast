package worker

import (
	"github.com/tidwall/gjson"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func getDuration(file string) (float64, error) {
	probe, err := ffmpeg.Probe(file)
	if err != nil {
		return 0, err
	}
	return gjson.Get(probe, "format.duration").Float(), nil
}
