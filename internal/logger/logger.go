package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/rs/zerolog"
)

func getSLogLevel(cfg config.LoggingConfig) slog.Level {
	switch cfg.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// This is faster than console logger
func NewJsonLogger(cfg config.LoggingConfig) *slog.Logger {
	zerologLogger := zerolog.New(os.Stdout).Level(toZerologLevel(getSLogLevel(cfg))).With().Timestamp().Logger()
	return slog.New(newZerologHandler(&zerologLogger))
}

func NewConsoleLogger(cfg config.LoggingConfig) *slog.Logger {
	zerologLogger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		NoColor:    false,
		TimeFormat: time.RFC3339Nano,
	}).Level(toZerologLevel(getSLogLevel(cfg))).With().Timestamp().Logger()
	return slog.New(newZerologHandler(&zerologLogger))
}

func New(cfg config.LoggingConfig) *slog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	switch cfg.LogFormat {
	case "json":
		return NewJsonLogger(cfg)
	default:
		return NewConsoleLogger(cfg)
	}
}
