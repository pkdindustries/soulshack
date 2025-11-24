package core

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

func InitLogger(verbose bool) {
	var config zap.Config

	if verbose {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		config.Encoding = "console" // Use console encoding for readability in IRC bot context
		// Enable colors for production logs too
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		// Simplified time format
		config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	}

	// Disable stacktrace for normal logs to keep output clean
	config.DisableStacktrace = !verbose

	l, err := config.Build()
	if err != nil {
		panic(err)
	}

	// Replace global logger
	zap.ReplaceGlobals(l)
	zap.RedirectStdLog(l)
	logger = l.Sugar()
}

// GetLogger returns the global sugared logger
func GetLogger() *zap.SugaredLogger {
	if logger == nil {
		InitLogger(false) // Default to non-verbose if not initialized
	}
	return logger
}

// WithFields creates a logger with the given structured fields
func WithFields(fields ...interface{}) *zap.SugaredLogger {
	return GetLogger().With(fields...)
}

// LogDuration logs the duration of an operation
// Usage: defer LogDuration(logger, "operation_name", time.Now())
func LogDuration(logger *zap.SugaredLogger, operation string, start time.Time) {
	duration := time.Since(start)
	logger.With(
		"operation", operation,
		"duration_ms", duration.Milliseconds(),
	).Debugf("Completed %s in %v", operation, duration)
}

// WithTool creates a logger with tool execution context
func WithTool(logger *zap.SugaredLogger, toolName string, args map[string]any) *zap.SugaredLogger {
	return logger.With(
		"tool", toolName,
		"tool_args", args,
	)
}

// WithIRCContext creates a logger with IRC-specific context
func WithIRCContext(logger *zap.SugaredLogger, channel, user string) *zap.SugaredLogger {
	return logger.With(
		"channel", channel,
		"user", user,
	)
}
