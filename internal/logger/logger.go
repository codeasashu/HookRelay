package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/rs/zerolog"
)

func getSLogLevel() slog.Level {
	switch config.HRConfig.Logging.LogLevel {
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
func NewJsonLogger() *slog.Logger {
	zerologLogger := zerolog.New(os.Stdout).Level(toZerologLevel(getSLogLevel())).With().Timestamp().Logger()
	return slog.New(newZerologHandler(&zerologLogger))
}

func NewConsoleLogger() *slog.Logger {
	zerologLogger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		NoColor:    false,
		TimeFormat: time.RFC3339Nano,
	}).Level(toZerologLevel(getSLogLevel())).With().Timestamp().Logger()
	return slog.New(newZerologHandler(&zerologLogger))
}

func New() *slog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	switch config.HRConfig.Logging.LogFormat {
	case "json":
		return NewJsonLogger()
	default:
		return NewConsoleLogger()
	}
}
