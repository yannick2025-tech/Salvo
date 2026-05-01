package logger

import (
	"context"
	"io"
	"os"
)

type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)

type LogFormat string

const (
	FormatText LogFormat = "text"
	FormatJSON LogFormat = "json"
)

type Config struct {
	Level      LogLevel `yaml:"level"`
	Format     LogFormat `yaml:"format"`
	Output     string    `yaml:"output"`
	TimeFormat string    `yaml:"time_format"`
}

type Field struct {
	Key   string
	Value any
}

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	With(fields ...Field) Logger
	WithContext(ctx context.Context) Logger

	Sync() error
}

func New(cfg Config) (Logger, error) {
	var w io.Writer = os.Stdout
	if cfg.Output != "" {
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		w = f
	}

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

func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}
