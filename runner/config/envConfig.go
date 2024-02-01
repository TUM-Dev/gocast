package config

import (
	"github.com/caarlos0/env"
	"log/slog"
)

type EnvConfig struct {
	LogFmt       string `env:"LOG_FMT" envDefault:"txt"`
	LogLevel     string `env:"LOG_LEVEL" envDefault:"debug"`
	Port         int    `env:"PORT" envDefault:"0"`
	StoragePath  string `env:"STORAGE_PATH" envDefault:"storage/mass"`
	SegmentPath  string `env:"SEGMENT_PATH" envDefault:"storage/live"`
	RecPath      string `env:"REC_PATH" envDefault:"storage/rec"`
	GocastServer string `env:"GOCAST_SERVER" envDefault:"localhost:50056"`
	Hostname     string `env:"REALHOST" envDefault:"localhost"`
	Version      string `env:"VERSION" envDefault:"dev"`
}

var Cfg *EnvConfig

func Init(log *slog.Logger) {

	log.Info("Initializing envConfig")
	if err := env.Parse(&Cfg); err != nil {
		log.Error("error parsing envConfig", "error", err)
	}
	log.Info("envConfig loaded", "envConfig", Cfg)
}
