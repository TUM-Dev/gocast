package worker

import (
	"errors"
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
	if codecType == "video" {
		for i := 0; i < int(nStreams); i++ {
			if gjson.Get(probe, fmt.Sprintf("streams.%d.codec_type", i)).String() == "video" {
				return gjson.Get(probe, fmt.Sprintf("streams.%d.codec_name", i)).String(), nil
			}
		}
		return "", errors.New("no video stream found")
	}
	if codecType == "audio" {
		for i := 0; i < int(nStreams); i++ {
			if gjson.Get(probe, fmt.Sprintf("streams.%d.codec_type", i)).String() == "audio" {
				return gjson.Get(probe, fmt.Sprintf("streams.%d.codec_name", i)).String(), nil
			}
		}
		return "", errors.New("no audio stream found")
	}
	return "", errors.New("no stream found")
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
