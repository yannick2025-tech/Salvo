package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T, format LogFormat) (*zapLogger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	cfg := Config{
		Level:      DebugLevel,
		Format:     format,
		Output:     "",
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}
	l, err := newZapLogger(cfg, &buf)
	require.NoError(t, err)
	return l, &buf
}

func TestLoggerJSONFormat(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	l.Info("hello", F("key1", "value1"), F("key2", 42))

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "value1", entry["key1"])
	assert.Equal(t, float64(42), entry["key2"])
	assert.Contains(t, entry, "ts")
}

func TestLoggerTextFormat(t *testing.T) {
	l, buf := newTestLogger(t, FormatText)

	l.Info("hello text", F("user", "alice"))

	output := buf.String()
	assert.Contains(t, output, "hello text")
	assert.Contains(t, output, "alice")
}

func TestLoggerAllLevels(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	tests := []struct {
		name  string
		logFn func(string, ...Field)
		level string
	}{
		{name: "debug", logFn: l.Debug, level: "debug"},
		{name: "info", logFn: l.Info, level: "info"},
		{name: "warn", logFn: l.Warn, level: "warn"},
		{name: "error", logFn: l.Error, level: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFn("test message", F("level_test", tt.name))

			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, tt.level, entry["level"])
			assert.Equal(t, "test message", entry["msg"])
		})
	}
}

func TestLoggerWithFields(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	child := l.With(F("service", "salvo"))
	child.Info("child message")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "salvo", entry["service"])
	assert.Equal(t, "child message", entry["msg"])
}

func TestLoggerWithFieldsChained(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	child := l.With(F("service", "salvo"))
	grandchild := child.With(F("version", "0.1.0"))
	grandchild.Info("nested message")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "salvo", entry["service"])
	assert.Equal(t, "0.1.0", entry["version"])
}

func TestLoggerWithContextTraceID(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	ctx := ContextWithTraceID(context.Background(), "trace-abc-123")
	child := l.WithContext(ctx)
	child.Info("with trace")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "trace-abc-123", entry["trace_id"])
	assert.Equal(t, "with trace", entry["msg"])
}

func TestLoggerWithContextNoTraceID(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	child := l.WithContext(context.Background())
	child.Info("no trace")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "no trace", entry["msg"])
	_, hasTraceID := entry["trace_id"]
	assert.False(t, hasTraceID)
}

func TestLoggerWithContextNil(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	child := l.WithContext(nil)
	child.Info("nil context")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "nil context", entry["msg"])
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:      WarnLevel,
		Format:     FormatJSON,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}
	l, err := newZapLogger(cfg, &buf)
	require.NoError(t, err)

	l.Debug("should not appear")
	l.Info("should not appear either")
	l.Warn("should appear")

	output := buf.String()
	assert.NotContains(t, output, "should not appear")
	assert.Contains(t, output, "should appear")
}

func TestTraceIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{name: "with trace id", ctx: ContextWithTraceID(context.Background(), "abc"), expected: "abc"},
		{name: "without trace id", ctx: context.Background(), expected: ""},
		{name: "nil context", ctx: nil, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TraceIDFromContext(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewLoggerDefaultConfig(t *testing.T) {
	l, err := New(Config{})
	require.NoError(t, err)
	assert.NotNil(t, l)
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	_, err := New(Config{Level: "invalid"})
	assert.Error(t, err)
}

func TestNewLoggerFileOutput(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	l, err := New(Config{
		Level:      InfoLevel,
		Format:     FormatJSON,
		Output:     tmpFile,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	require.NoError(t, err)

	l.Info("file output test")
	require.NoError(t, l.Sync())

	data, err := osReadFile(tmpFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "file output test")
}

func TestLoggerSync(t *testing.T) {
	l, _ := newTestLogger(t, FormatJSON)
	err := l.Sync()
	assert.NoError(t, err)
}

func TestLoggerMultipleFields(t *testing.T) {
	l, buf := newTestLogger(t, FormatJSON)

	l.Info("multi fields",
		F("str", "hello"),
		F("int", 123),
		F("bool", true),
		F("float", 3.14),
	)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "hello", entry["str"])
	assert.Equal(t, float64(123), entry["int"])
	assert.Equal(t, true, entry["bool"])
	assert.InDelta(t, 3.14, entry["float"], 0.001)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    LogLevel
		wantErr  bool
	}{
		{input: DebugLevel, wantErr: false},
		{input: InfoLevel, wantErr: false},
		{input: WarnLevel, wantErr: false},
		{input: ErrorLevel, wantErr: false},
		{input: FatalLevel, wantErr: false},
		{input: LogLevel("unknown"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			_, err := parseLevel(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func osReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
