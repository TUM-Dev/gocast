package config

import (
	"log/slog"

	"github.com/caarlos0/env"
)

var Config struct {
	LogFmt       string `env:"LOG_FMT" envDefault:"txt"`
	LogLevel     string `env:"LOG_LEVEL" envDefault:"debug"`
	Port         int    `env:"PORT" envDefault:"52735"`
	StoragePath  string `env:"STORAGE_PATH" envDefault:"storage/mass"`
	SegmentPath  string `env:"SEGMENT_PATH" envDefault:"storage/live"`
	ErrorPath    string `env:"ERROR_PATH" envDefault:"storage/errors"`
	GocastServer string `env:"GOCAST_SERVER" envDefault:"localhost:50056"`
	Hostname     string `env:"REALHOST" envDefault:"localhost"`
	EdgeServer   string `env:"EDGE_SERVER" envDefault:"http://localhost:8089"`
	// Token is the shared secret from gocast's config.yaml (runnerToken). The runner sends it
	// with every call to gocast and requires it on every call gocast makes to the runner.
	// There is no default on purpose: a runner without a token refuses to start.
	Token string `env:"TOKEN"`
}

func init() {
	if err := env.Parse(&Config); err != nil {
		slog.Error("error parsing envConfig", "error", err)
	}

	// Log a redacted copy; the token is a credential.
	redacted := Config
	if redacted.Token != "" {
		redacted.Token = "<set>"
	}
	slog.Info("envConfig loaded", "envConfig", redacted)
}
