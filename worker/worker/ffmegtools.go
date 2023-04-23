package worker

import (
	"github.com/tidwall/gjson"
	"os/exec"
)

func getDuration(file string) (float64, error) {
	probe, err := probe(file)

	if err != nil {
		return 0, err
	}
	return gjson.Get(probe, "format.duration").Float(), nil
}

func getCodec(file string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	return gjson.Get(probe, "streams.0.codec_name").String(), nil
}

func getLevel(file string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	return gjson.Get(probe, "streams.0.level").String(), nil
}

func getContainer(file string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	return gjson.Get(probe, "format.format_name").String(), nil
}

func probe(file string) (string, error) {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams", file).CombinedOutput()
	return string(out), err
}
