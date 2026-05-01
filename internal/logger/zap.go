package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is the type used for context value keys in this package.
type contextKey string

// traceIDKey is the context key for the trace identifier.
const traceIDKey contextKey = "trace_id"

// ContextWithTraceID returns a copy of ctx that carries the given traceID.
// The trace ID will be automatically injected into log entries created via
// WithContext.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext extracts the trace ID from ctx.
// Returns an empty string if ctx is nil or no trace ID is present.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// zapLogger is the zap-backed implementation of the Logger interface.
type zapLogger struct {
	sugar *zap.SugaredLogger
	base  *zap.Logger
	level zapcore.Level
}

// newZapLogger constructs a zapLogger writing to w with the given Config.
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

// Debug logs a message at debug severity.
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.sugar.Debugw(msg, toZapFields(fields)...)
}

// Info logs a message at info severity.
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.sugar.Infow(msg, toZapFields(fields)...)
}

// Warn logs a message at warn severity.
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.sugar.Warnw(msg, toZapFields(fields)...)
}

// Error logs a message at error severity.
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.sugar.Errorw(msg, toZapFields(fields)...)
}

// Fatal logs a message at fatal severity and terminates the process.
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.sugar.Fatalw(msg, toZapFields(fields)...)
}

// With returns a child Logger that always includes the given fields.
func (l *zapLogger) With(fields ...Field) Logger {
	newSugar := l.sugar.With(toZapFields(fields)...)
	return &zapLogger{
		sugar: newSugar,
		base:  l.base,
		level: l.level,
	}
}

// WithContext returns a child Logger that injects the trace-id from ctx.
func (l *zapLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		return l.With(F("trace_id", traceID))
	}
	return l
}

// Sync flushes any buffered log entries.
func (l *zapLogger) Sync() error {
	return l.sugar.Sync()
}

// parseLevel converts a LogLevel string to a zapcore.Level.
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

// toZapFields converts a slice of Field to the alternating key-value
// slice expected by zap's SugaredLogger.
func toZapFields(fields []Field) []any {
	result := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		result = append(result, f.Key, f.Value)
	}
	return result
}

func init() {
	// Ensure os.Stderr.Sync is available for zap's error output sink.
	_ = os.Stderr.Sync
}
