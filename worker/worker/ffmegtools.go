package worker

import (
	"os/exec"

	"github.com/tidwall/gjson"
)

func getDuration(file string) (float64, error) {
	probe, err := probe(file)
	if err != nil {
		return 0, err
	}
	return gjson.Get(probe, "format.duration").Float(), nil
}

func probe(file string) (string, error) {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams", file).CombinedOutput()
	return string(out), err
}
