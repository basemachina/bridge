package main

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewStackdriver creates a new zap logger for stackdriver
func NewStackdriver(level string) (*zap.Logger, error) {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(logLevel(level)),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			LevelKey:      "severity",
			NameKey:       "logger",
			CallerKey:     "caller",
			StacktraceKey: "stack_trace",
			TimeKey:       "time",
			MessageKey:    "message",
			LineEnding:    zapcore.DefaultLineEnding,
			EncodeTime:    zapcore.RFC3339NanoTimeEncoder,
			EncodeLevel:   levelEncode,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		},
		DisableStacktrace: true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}.Build()
}

func logLevel(level string) zapcore.Level {
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		return zapcore.DebugLevel
	case "WARN":
		return zapcore.WarnLevel
	case "ERROR":
		return zapcore.ErrorLevel
	}
	return zapcore.InfoLevel
}

// severityName is copied from these lines.
// https://github.com/googleapis/google-cloud-go/blob/6637d0c93932ed839229391d7796785cc4d7cc21/logging/logging.go#L440-L450
func levelEncode(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString("Debug")
	case zapcore.InfoLevel:
		enc.AppendString("Info")
	case zapcore.WarnLevel:
		enc.AppendString("Warning")
	case zapcore.ErrorLevel:
		enc.AppendString("Error")
	default:
		enc.AppendString("Critical")
	}
}
