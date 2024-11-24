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

func getVideoCodec(file string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	codecNumber := gjson.Get(probe, "streams.#").Int()
	videoIndex := -1
	for i := 0; i < int(codecNumber); i++ {
		if gjson.Get(probe, fmt.Sprintf("streams.%d.codec_type", i)).String() == "video" {
			videoIndex = int(gjson.Get(probe, fmt.Sprintf("streams.%d.index", i)).Int())
			break
		}
	}
	if videoIndex != -1 {
		return gjson.Get(probe, fmt.Sprintf("streams.%d.codec_name", videoIndex)).String(), nil
	}
	return "", errors.New("no video stream found")
}

func getAudioCodec(file string) (string, error) {
	probe, err := probe(file)
	if err != nil {
		return "", err
	}
	codecNumber := gjson.Get(probe, "streams.#").Int()
	audioIndex := -1
	for i := 0; i < int(codecNumber); i++ {
		if gjson.Get(probe, fmt.Sprintf("streams.%d.codec_type", i)).String() == "audio" {
			audioIndex = int(gjson.Get(probe, fmt.Sprintf("streams.%d.index", i)).Int())
			break
		}
	}
	if audioIndex != -1 {
		return gjson.Get(probe, fmt.Sprintf("streams.%d.codec_name", audioIndex)).String(), nil
	}
	return "", errors.New("no video stream found")
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
