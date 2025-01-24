package config

import (
	"github.com/caarlos0/env"
	"log/slog"
)

var Config struct {
	LogFmt       string `env:"LOG_FMT" envDefault:"txt"`
	LogLevel     string `env:"LOG_LEVEL" envDefault:"debug"`
	Port         int    `env:"PORT" envDefault:"0"`
	StoragePath  string `env:"STORAGE_PATH" envDefault:"storage/mass"`
	SegmentPath  string `env:"SEGMENT_PATH" envDefault:"storage/live"`
	GocastServer string `env:"GOCAST_SERVER" envDefault:"localhost:50056"`
	Hostname     string `env:"REALHOST" envDefault:"localhost"`
	Version      string `env:"VERSION" envDefault:"dev"`
	EdgeServer   string `env:"EDGE_SERVER" envDefault:"localhost:50057"`
}

func init() {
	if err := env.Parse(&Config); err != nil {
		slog.Error("error parsing envConfig", "error", err)
	}

	slog.Info("envConfig loaded", "envConfig", Config)
}
