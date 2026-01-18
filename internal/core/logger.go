package core

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

var logger *slog.Logger

// InitLogger initializes the global logger with level and format.
// level: debug, info, warn, error (default: info)
// format: text (colorized tint), json (default: text)
func InitLogger(level, format string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slogLevel,
		})
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slogLevel,
			TimeFormat: "15:04:05",
		})
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// GetLogger returns the global logger
func GetLogger() *slog.Logger {
	if logger == nil {
		InitLogger("info", "text")
	}
	return logger
}

// WithFields creates a logger with the given structured fields
func WithFields(args ...any) *slog.Logger {
	return GetLogger().With(args...)
}

// LogDuration logs the duration of an operation
// Usage: defer LogDuration(logger, "operation_name", time.Now())
func LogDuration(logger *slog.Logger, operation string, start time.Time) {
	duration := time.Since(start)
	logger.Debug("operation completed",
		"operation", operation,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
	)
}

// WithTool creates a logger with tool execution context
func WithTool(logger *slog.Logger, toolName string, args map[string]any) *slog.Logger {
	return logger.With(
		"tool", toolName,
		"tool_args", args,
	)
}
