package config

type Config struct {
	Manager     string `yaml:"manager"`
	StoragePath string `yaml:"storagePath"`
	TmpPath     string `yaml:"tmpPath"`
	Stream      struct {
		CodecA             string `yaml:"codecA"`
		CodecV             string `yaml:"codecV"`
		HlsSegmentDuration int    `yaml:"hlsSegmentDuration"`
		HlsPlaylistLength  int    `yaml:"hlsPlaylistLength"`
		MaxDuration        int    `yaml:"maxDuration"`
	} `yaml:"stream"`
	Transcode struct {
		CodecA   string   `yaml:"codecA"`
		CodecV   string   `yaml:"codecV"`
		Crf      int      `yaml:"crf"`
		Level    float64  `yaml:"level"`
		Movflags []string `yaml:"movflags"`
	} `yaml:"transcode"`
}
