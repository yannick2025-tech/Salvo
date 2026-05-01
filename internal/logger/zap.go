package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const traceIDKey contextKey = "trace_id"

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

type zapLogger struct {
	sugar *zap.SugaredLogger
	base  *zap.Logger
	level zapcore.Level
}

func newZapLogger(cfg Config, w io.Writer) (*zapLogger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(cfg.TimeFormat),
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == FormatText {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(w),
		level,
	)

	base := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{
		sugar: base.Sugar(),
		base:  base,
		level: level,
	}, nil
}

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.sugar.Debugw(msg, toZapFields(fields)...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.sugar.Infow(msg, toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.sugar.Warnw(msg, toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, fields ...Field) {
	l.sugar.Errorw(msg, toZapFields(fields)...)
}

func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.sugar.Fatalw(msg, toZapFields(fields)...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	newSugar := l.sugar.With(toZapFields(fields)...)
	return &zapLogger{
		sugar: newSugar,
		base:  l.base,
		level: l.level,
	}
}

func (l *zapLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		return l.With(F("trace_id", traceID))
	}
	return l
}

func (l *zapLogger) Sync() error {
	return l.sugar.Sync()
}

func parseLevel(level LogLevel) (zapcore.Level, error) {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel, nil
	case InfoLevel:
		return zapcore.InfoLevel, nil
	case WarnLevel:
		return zapcore.WarnLevel, nil
	case ErrorLevel:
		return zapcore.ErrorLevel, nil
	case FatalLevel:
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

func toZapFields(fields []Field) []any {
	result := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		result = append(result, f.Key, f.Value)
	}
	return result
}

func init() {
	_ = os.Stderr.Sync
}
