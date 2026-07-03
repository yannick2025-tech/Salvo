package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

func TestParseRetryConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected *nodeRetryConfig
	}{
		{
			name:     "empty config",
			config:   `{}`,
			expected: nil,
		},
		{
			name:   "basic retry config",
			config: `{"retry": {"max_attempts": 3}}`,
			expected: &nodeRetryConfig{
				MaxAttempts:    3,
				InitialBackoff: "100ms",
				Multiplier:     2.0,
				MaxBackoff:     "30s",
				Jitter:         false,
				OnStatus:       []int{429, 503},
			},
		},
		{
			name: "full retry config",
			config: `{
				"retry": {
					"max_attempts": 5,
					"initial_backoff": "200ms",
					"multiplier": 3.0,
					"max_backoff": "1m",
					"jitter": true,
					"on_status": [429, 500, 503]
				}
			}`,
			expected: &nodeRetryConfig{
				MaxAttempts:    5,
				InitialBackoff: "200ms",
				Multiplier:     3.0,
				MaxBackoff:     "1m",
				Jitter:         true,
				OnStatus:       []int{429, 500, 503},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &sceneNode{
				config: tt.config,
			}
			result := node.parseRetryConfig()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.MaxAttempts != tt.expected.MaxAttempts {
				t.Errorf("MaxAttempts: expected %d, got %d", tt.expected.MaxAttempts, result.MaxAttempts)
			}
			if result.InitialBackoff != tt.expected.InitialBackoff {
				t.Errorf("InitialBackoff: expected %s, got %s", tt.expected.InitialBackoff, result.InitialBackoff)
			}
			if result.Multiplier != tt.expected.Multiplier {
				t.Errorf("Multiplier: expected %f, got %f", tt.expected.Multiplier, result.Multiplier)
			}
			if result.MaxBackoff != tt.expected.MaxBackoff {
				t.Errorf("MaxBackoff: expected %s, got %s", tt.expected.MaxBackoff, result.MaxBackoff)
			}
			if result.Jitter != tt.expected.Jitter {
				t.Errorf("Jitter: expected %v, got %v", tt.expected.Jitter, result.Jitter)
			}
			if len(result.OnStatus) != len(tt.expected.OnStatus) {
				t.Errorf("OnStatus length: expected %d, got %d", len(tt.expected.OnStatus), len(result.OnStatus))
			}
		})
	}
}

func TestParseExtractConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected []nodeExtractEntry
	}{
		{
			name:     "empty config",
			config:   `{}`,
			expected: nil,
		},
		{
			name:   "object format",
			config: `{"extract": {"token": "$.data.token", "user_id": "$.data.userId"}}`,
			expected: []nodeExtractEntry{
				{Variable: "token", Path: "$.data.token"},
				{Variable: "user_id", Path: "$.data.userId"},
			},
		},
		{
			name: "array format",
			config: `{
				"extract": [
					{"variable": "status", "path": "$.result.status"},
					{"variable": "code", "path": "$.result.code"}
				]
			}`,
			expected: []nodeExtractEntry{
				{Variable: "status", Path: "$.result.status"},
				{Variable: "code", Path: "$.result.code"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &sceneNode{
				config: tt.config,
			}
			result := node.parseExtractConfig()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d", len(tt.expected), len(result))
			}

			// Create maps for easier comparison (order doesn't matter for object format)
			resultMap := make(map[string]string)
			for _, entry := range result {
				resultMap[entry.Variable] = entry.Path
			}

			expectedMap := make(map[string]string)
			for _, entry := range tt.expected {
				expectedMap[entry.Variable] = entry.Path
			}

			for varName, expectedPath := range expectedMap {
				actualPath, exists := resultMap[varName]
				if !exists {
					t.Errorf("missing variable %s", varName)
					continue
				}
				if actualPath != expectedPath {
					t.Errorf("variable %s: expected path %s, got %s", varName, expectedPath, actualPath)
				}
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name        string
		config      *nodeRetryConfig
		attempt     int
		expectMin   time.Duration
		expectMax   time.Duration
	}{
		{
			name: "first retry without jitter",
			config: &nodeRetryConfig{
				InitialBackoff: "100ms",
				Multiplier:     2.0,
				MaxBackoff:     "30s",
				Jitter:         false,
			},
			attempt:   0,
			expectMin: 100 * time.Millisecond,
			expectMax: 100 * time.Millisecond,
		},
		{
			name: "second retry without jitter",
			config: &nodeRetryConfig{
				InitialBackoff: "100ms",
				Multiplier:     2.0,
				MaxBackoff:     "30s",
				Jitter:         false,
			},
			attempt:   1,
			expectMin: 200 * time.Millisecond,
			expectMax: 200 * time.Millisecond,
		},
		{
			name: "third retry without jitter",
			config: &nodeRetryConfig{
				InitialBackoff: "100ms",
				Multiplier:     2.0,
				MaxBackoff:     "30s",
				Jitter:         false,
			},
			attempt:   2,
			expectMin: 400 * time.Millisecond,
			expectMax: 400 * time.Millisecond,
		},
		{
			name: "exceeds max backoff",
			config: &nodeRetryConfig{
				InitialBackoff: "1s",
				Multiplier:     10.0,
				MaxBackoff:     "5s",
				Jitter:         false,
			},
			attempt:   2,
			expectMin: 5 * time.Second,
			expectMax: 5 * time.Second,
		},
		{
			name: "with jitter",
			config: &nodeRetryConfig{
				InitialBackoff: "100ms",
				Multiplier:     2.0,
				MaxBackoff:     "30s",
				Jitter:         true,
			},
			attempt:   0,
			expectMin: 50 * time.Millisecond,  // 100ms * 0.5
			expectMax: 150 * time.Millisecond, // 100ms * 1.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBackoff(tt.config, tt.attempt)

			if result < tt.expectMin || result > tt.expectMax {
				t.Errorf("expected backoff between %v and %v, got %v",
					tt.expectMin, tt.expectMax, result)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		config   *nodeRetryConfig
		err      error
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "nil error",
			config:   &nodeRetryConfig{OnStatus: []int{429}},
			err:      nil,
			expected: false,
		},
		{
			name: "matching status 429",
			config: &nodeRetryConfig{
				OnStatus: []int{429, 503},
			},
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name: "matching status in error message",
			config: &nodeRetryConfig{
				OnStatus: []int{429, 503},
			},
			err:      context.DeadlineExceeded,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRetry(tt.config, tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestApplyExtract(t *testing.T) {
	tests := []struct {
		name     string
		output   *dag.Output
		extracts []nodeExtractEntry
		vars     map[string]any
		expected map[string]any
	}{
		{
			name: "extract from map response",
			output: &dag.Output{
				Response: map[string]any{
					"data": map[string]any{
						"token":  "abc123",
						"userId": 456,
					},
				},
			},
			extracts: []nodeExtractEntry{
				{Variable: "token", Path: "$.data.token"},
				{Variable: "user_id", Path: "$.data.userId"},
			},
			vars: make(map[string]any),
			expected: map[string]any{
				"token":   "abc123",
				"user_id": 456,
			},
		},
		{
			name: "extract from JSON string response",
			output: &dag.Output{
				Response: `{"data": {"status": "success", "code": 200}}`,
			},
			extracts: []nodeExtractEntry{
				{Variable: "status", Path: "$.data.status"},
				{Variable: "code", Path: "$.data.code"},
			},
			vars: make(map[string]any),
			expected: map[string]any{
				"status": "success",
				"code":   200,
			},
		},
		{
			name: "extract from byte array response",
			output: &dag.Output{
				Response: []byte(`{"result": {"value": 123}}`),
			},
			extracts: []nodeExtractEntry{
				{Variable: "value", Path: "$.result.value"},
			},
			vars: make(map[string]any),
			expected: map[string]any{
				"value": 123,
			},
		},
		{
			name: "missing path",
			output: &dag.Output{
				Response: map[string]any{
					"data": map[string]any{
						"token": "abc123",
					},
				},
			},
			extracts: []nodeExtractEntry{
				{Variable: "token", Path: "$.data.token"},
				{Variable: "missing", Path: "$.data.missing"},
			},
			vars: make(map[string]any),
			expected: map[string]any{
				"token": "abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &sceneNode{}
			// Create a minimal DAG and Executor for testing
			d := dag.New()
			// Use WithInitialVars to pass tt.vars as initialVars, so SetVariable writes to tt.vars
			exec := dag.NewExecutor(d, dag.WithInitialVars(tt.vars))
			input := &dag.Input{
				Variables: tt.vars,
				Executor:  exec,
			}
			log, _ := logger.New(logger.Config{Level: logger.DebugLevel})

			node.applyExtract(tt.output, tt.extracts, input, log)

			// Check results
			for varName, expectedValue := range tt.expected {
				actualValue, exists := tt.vars[varName]
				if !exists {
					t.Errorf("variable %s not set", varName)
					continue
				}
				// Use fmt.Sprintf for comparison to handle float64 vs int type differences
				// from JSON unmarshaling
				if fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", expectedValue) {
					t.Errorf("variable %s: expected %v, got %v", varName, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestResolveJSONPath(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		path     string
		expected any
	}{
		{
			name: "simple field",
			data: map[string]any{
				"token": "abc123",
			},
			path:     "$.token",
			expected: "abc123",
		},
		{
			name: "nested field",
			data: map[string]any{
				"data": map[string]any{
					"user": map[string]any{
						"name": "John",
					},
				},
			},
			path:     "$.data.user.name",
			expected: "John",
		},
		{
			name: "array index",
			data: map[string]any{
				"items": []any{"a", "b", "c"},
			},
			path:     "$.items[1]",
			expected: "b",
		},
		{
			name: "nested array",
			data: map[string]any{
				"data": map[string]any{
					"items": []any{
						map[string]any{"id": 1},
						map[string]any{"id": 2},
					},
				},
			},
			path:     "$.data.items[1].id",
			expected: 2,
		},
		{
			name: "missing field",
			data: map[string]any{
				"data": map[string]any{
					"token": "abc",
				},
			},
			path:     "$.data.missing",
			expected: nil,
		},
		{
			name: "invalid path",
			data: map[string]any{
				"token": "abc",
			},
			path:     "invalid.path",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveJSONPath(tt.data, tt.path)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "milliseconds",
			input:    "100ms",
			expected: 100 * time.Millisecond,
			hasError: false,
		},
		{
			name:     "seconds",
			input:    "5s",
			expected: 5 * time.Second,
			hasError: false,
		},
		{
			name:     "minutes",
			input:    "1m",
			expected: 1 * time.Minute,
			hasError: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			hasError: true,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
