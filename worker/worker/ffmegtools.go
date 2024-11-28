package worker

import (
	"fmt"
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

func getCodec(file string, codecType string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	nStreams := gjson.Get(probe, "streams.#").Int()
	for i := 0; i < int(nStreams); i++ {
		if gjson.Get(probe, fmt.Sprintf("streams.%d.codec_type", i)).String() == codecType {
			return gjson.Get(probe, fmt.Sprintf("streams.%d.codec_name", i)).String(), nil
		}
	}
	return "", fmt.Errorf("no %s stream found", codecType)
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
