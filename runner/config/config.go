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
	UploadPath   string `env:"UPLOAD_PATH" envDefault:"storage/uploads"`
	UploadPort   int    `env:"UPLOAD_PORT" envDefault:"8188"`
	GocastServer string `env:"GOCAST_SERVER" envDefault:"localhost:50056"`
	Hostname     string `env:"REALHOST" envDefault:"localhost"`
	EdgeServer   string `env:"EDGE_SERVER" envDefault:"http://localhost:8089"`
	// TLS* switch the link to gocast to mutual TLS. All three set, or none (development).
	TLSCert string `env:"TLS_CERT"`
	TLSKey  string `env:"TLS_KEY"`
	TLSCA   string `env:"TLS_CA"`
	// MaxPostProcessing is how many thumbnail regenerations, sprites and silence scans run at once.
	MaxPostProcessing int `env:"MAX_POSTPROCESSING" envDefault:"3"`
}

func init() {
	if err := env.Parse(&Config); err != nil {
		slog.Error("error parsing envConfig", "error", err)
	}

	slog.Info("envConfig loaded", "envConfig", Config)
}
