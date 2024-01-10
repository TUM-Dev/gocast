package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/dusted-go/logging/prettylog"
)

// GetLogger creates a *slog.Logger based on the environment, sets it as the default logger and returns it.
func GetLogger(v string) *slog.Logger {
	lvlStr := os.Getenv("LOG_LEVEL")
	var level slog.Level
	switch strings.ToLower(lvlStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}
	var logger *slog.Logger
	if os.Getenv("LOG_FMT") == "json" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		}))
	} else {
		logger = slog.New(prettylog.NewHandler(&slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				return a // don't replace any attributes
			},
		}))
	}
	slog.SetDefault(logger)
	return logger
}
