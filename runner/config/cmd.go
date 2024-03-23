package config

import (
	"github.com/ghodss/yaml"
	"log/slog"
	"os"
	"path/filepath"
)

type CmdList struct {
	//this is for adding extra parameters
	Stream            string `Default:"-y -hide_banner -nostats %x -t &.0f -i %s -c:v copy -c:a copy -f mpegts %x -c:v libx264 -preset veryfast -tune zerolatency -maxrate 2500k -bufsize 3000k -g 60 -r 30 -x264-params keyint=60:scenecut=0 -c:a aac -ar 44100 -b:a 128k -f hls -hls_time 2 -hls_list_size 3600 -hls_playlist_type event -hls_flags append_list -hls_segment_filename %x %x"`
	SeparateAudioFast string `Default:"-i %v -vn -c:a copy %v"`
	SeparateAudio     string `Default:"-i %v -vn %v"`
	AudioNormalize1   string `Default:"-i %v -nostats -y -af loudnorm=I=-23:TP=-2:LRA=7:print_format=json -f null -"`
	AudioNormalize2   string `Default:"-i %v -af loudnorm=I=-23:TP=-2:LRA=7:measured_i=%v:measured_tp=%v:measured_lra=%v:measured_thresh=%v:offset=%v:linear=true:print_format=summary -c:a aac -c:v libx264 -crf 0 -probesize 100M -analyzeduration 250M %v"`
	Transcoding       string `Default:"-i %v -c:v libx264 %v"`
}

func NewCmd(log *slog.Logger) *CmdList {
	var c CmdList
	path, _ := filepath.Abs("cmd.yaml")
	YamlFile, err := os.ReadFile(path)
	if err != nil {
		log.Error("error reading cmd.yaml", "error", err)
	}

	if yaml.Unmarshal(YamlFile, &c) != nil {
		log.Error("error unmarshalling cmd.yaml", "error", err)
	}
	log.Info("cmd loaded", "cmd", &c)
	return &c
}
