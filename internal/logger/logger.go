// Package logger provides a structured logging abstraction backed by zap.
//
// It supports two output formats (text and JSON), configurable log levels,
// and automatic trace-id injection from context. The Logger interface is
// the primary contract; callers should depend on it rather than the concrete
// zap implementation.
package logger

import (
	"context"
	"io"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel represents the severity level of a log entry.
type LogLevel string

const (
	// DebugLevel logs are typically voluminous and disabled in production.
	DebugLevel LogLevel = "debug"
	// InfoLevel is the default level for operational messages.
	InfoLevel LogLevel = "info"
	// WarnLevel logs indicate potential issues that are not errors.
	WarnLevel LogLevel = "warn"
	// ErrorLevel logs indicate failures that should be investigated.
	ErrorLevel LogLevel = "error"
	// FatalLevel logs cause the process to exit after the message is written.
	FatalLevel LogLevel = "fatal"
)

// LogFormat controls the output encoding of log entries.
type LogFormat string

const (
	// FormatText produces human-readable console output.
	FormatText LogFormat = "text"
	// FormatJSON produces machine-parseable JSON output.
	FormatJSON LogFormat = "json"
)

// Config holds the configuration for constructing a Logger.
type Config struct {
	Level      LogLevel  `yaml:"level"`
	Format     LogFormat `yaml:"format"`
	Output     string    `yaml:"output"`
	MaxSize    int       `yaml:"max_size"`
	MaxBackups int       `yaml:"max_backups"`
	MaxAge     int       `yaml:"max_age"`
	Compress   bool      `yaml:"compress"`
	TimeFormat string    `yaml:"time_format"`
}

// Field is a key-value pair attached to a log entry.
type Field struct {
	Key   string
	Value any
}

// Logger is the core logging interface used throughout Salvo.
// Implementations must be safe for concurrent use.
type Logger interface {
	// Debug logs a message at debug severity.
	Debug(msg string, fields ...Field)
	// Info logs a message at info severity.
	Info(msg string, fields ...Field)
	// Warn logs a message at warn severity.
	Warn(msg string, fields ...Field)
	// Error logs a message at error severity.
	Error(msg string, fields ...Field)
	// Fatal logs a message at fatal severity and terminates the process.
	Fatal(msg string, fields ...Field)

	// With returns a child Logger that always includes the given fields.
	With(fields ...Field) Logger
	// WithContext returns a child Logger that injects the trace-id from ctx
	// (if present) into every subsequent log entry.
	WithContext(ctx context.Context) Logger

	// Sync flushes any buffered log entries. Call this before process exit.
	Sync() error
}

// New creates a Logger from the supplied Config.
// If Config fields are empty, sensible defaults are applied.
func New(cfg Config) (Logger, error) {
	var writers []io.Writer

	if cfg.Output != "" {
		maxSize := cfg.MaxSize
		if maxSize <= 0 {
			maxSize = 500 // Default to 500MB per log file
		}
		maxBackups := cfg.MaxBackups
		if maxBackups <= 0 {
			maxBackups = 5
		}
		maxAge := cfg.MaxAge
		if maxAge <= 0 {
			maxAge = 30
		}
		lj := &lumberjack.Logger{
			Filename:   cfg.Output,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   cfg.Compress, // Enable compression for rotated logs
		}
		writers = append(writers, lj)
		writers = append(writers, os.Stdout)
	} else {
		writers = append(writers, os.Stdout)
	}

	w := io.MultiWriter(writers...)

	if cfg.Level == "" {
		cfg.Level = InfoLevel
	}
	if cfg.Format == "" {
		cfg.Format = FormatJSON
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	}

	return newZapLogger(cfg, w)
}

// F is a convenience constructor for a log Field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}
