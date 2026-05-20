package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid count mode",
			cfg:  Config{SceneID: 1, Workers: 2, RunMode: RunModeCount, Count: 10},
		},
		{
			name: "valid duration mode",
			cfg:  Config{SceneID: 1, Workers: 2, RunMode: RunModeDuration, Duration: 5 * time.Second},
		},
		{
			name:    "missing scene_id",
			cfg:     Config{Workers: 2, RunMode: RunModeCount, Count: 10},
			wantErr: true,
		},
		{
			name:    "zero workers",
			cfg:     Config{SceneID: 1, Workers: 0, RunMode: RunModeCount, Count: 10},
			wantErr: true,
		},
		{
			name:    "count mode with zero count",
			cfg:     Config{SceneID: 1, Workers: 2, RunMode: RunModeCount, Count: 0},
			wantErr: true,
		},
		{
			name:    "duration mode with zero duration",
			cfg:     Config{SceneID: 1, Workers: 2, RunMode: RunModeDuration, Duration: 0},
			wantErr: true,
		},
		{
			name:    "invalid run mode",
			cfg:     Config{SceneID: 1, Workers: 2, RunMode: "invalid", Count: 10},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStatsRecordLatency(t *testing.T) {
	s := &Stats{}

	s.RecordLatency(10*time.Millisecond, true)
	s.RecordLatency(20*time.Millisecond, true)
	s.RecordLatency(5*time.Millisecond, false)

	assert.Equal(t, int64(3), s.TotalReqs.Load())
	assert.Equal(t, int64(2), s.SuccessReqs.Load())
	assert.Equal(t, int64(1), s.FailedReqs.Load())
}

func TestStatsLatencyPercentiles(t *testing.T) {
	s := &Stats{}

	for i := 0; i < 100; i++ {
		s.RecordLatency(time.Duration(i+1)*time.Millisecond, true)
	}

	avg, p50, p95, p99 := s.LatencyPercentiles()
	assert.True(t, avg > 0, "avg should be positive")
	assert.True(t, p50 > 0, "p50 should be positive")
	assert.True(t, p95 > 0, "p95 should be positive")
	assert.True(t, p99 > 0, "p99 should be positive")
	assert.True(t, p50 <= p95, "p50 <= p95")
	assert.True(t, p95 <= p99, "p95 <= p99")
}

func TestStatsEmptyPercentiles(t *testing.T) {
	s := &Stats{}
	avg, p50, p95, p99 := s.LatencyPercentiles()
	assert.Equal(t, time.Duration(0), avg)
	assert.Equal(t, time.Duration(0), p50)
	assert.Equal(t, time.Duration(0), p95)
	assert.Equal(t, time.Duration(0), p99)
}

func TestRunnerStatusTransition(t *testing.T) {
	r := &Runner{
		stats: &Stats{},
		done:  make(chan struct{}),
	}
	r.status.Store(StatusPending)
	assert.Equal(t, StatusPending, r.Status())

	r.status.Store(StatusRunning)
	assert.Equal(t, StatusRunning, r.Status())

	r.status.Store(StatusDone)
	assert.Equal(t, StatusDone, r.Status())
}

func TestRunnerDurationBeforeFinish(t *testing.T) {
	r := &Runner{
		stats: &Stats{},
		done:  make(chan struct{}),
	}
	r.startedAt = time.Now().Add(-5 * time.Second)
	r.finishedAt = time.Time{}

	dur := r.Duration()
	assert.True(t, dur >= 5*time.Second, "duration should be at least 5s")
}

func TestRunnerDurationAfterFinish(t *testing.T) {
	r := &Runner{
		stats: &Stats{},
		done:  make(chan struct{}),
	}
	r.startedAt = time.Now().Add(-10 * time.Second)
	r.finishedAt = time.Now().Add(-5 * time.Second)

	dur := r.Duration()
	assert.True(t, dur >= 4*time.Second && dur <= 6*time.Second, "duration should be ~5s")
}

func TestNewRunnerValidation(t *testing.T) {
	cfg := Config{Workers: 0, RunMode: RunModeCount, Count: 10}
	_, err := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scene_id is required")
}

func TestManagerStartDuplicate(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil)

	r := &Runner{
		cfg:   Config{SceneID: 42},
		stats: &Stats{},
		done:  make(chan struct{}),
	}
	m.runners[42] = r

	err := m.Stop(42)
	assert.NoError(t, err)
}

func TestManagerStopNotRunning(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil)
	err := m.Stop(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestManagerGetNotRunning(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil)
	_, ok := m.Get(999)
	assert.False(t, ok)
}

func TestManagerList(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil)
	r1 := &Runner{cfg: Config{SceneID: 1}, stats: &Stats{}, done: make(chan struct{})}
	r2 := &Runner{cfg: Config{SceneID: 2}, stats: &Stats{}, done: make(chan struct{})}
	m.runners[1] = r1
	m.runners[2] = r2

	list := m.List()
	assert.Len(t, list, 2)
}

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{"empty expression", "", nil, true},
		{"literal true", "true", nil, true},
		{"literal false", "false", nil, false},
		{"zero string", "0", nil, false},
		{"non-empty string", "hello", nil, true},
		{"empty single quotes", "''", nil, false},
		{"empty double quotes", "\"\"", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &dag.Input{Variables: tt.vars}
			got := evaluateExpression(tt.expr, input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalConditionIfElse(t *testing.T) {
	r := &Runner{}

	t.Run("if_true with true result", func(t *testing.T) {
		output := &dag.Output{Response: map[string]any{"if_else_result": true}}
		assert.True(t, r.evalCondition(nil, "__if_true__", output))
	})

	t.Run("if_true with false result", func(t *testing.T) {
		output := &dag.Output{Response: map[string]any{"if_else_result": false}}
		assert.False(t, r.evalCondition(nil, "__if_true__", output))
	})

	t.Run("if_false with false result", func(t *testing.T) {
		output := &dag.Output{Response: map[string]any{"if_else_result": false}}
		assert.True(t, r.evalCondition(nil, "__if_false__", output))
	})

	t.Run("if_false with true result", func(t *testing.T) {
		output := &dag.Output{Response: map[string]any{"if_else_result": true}}
		assert.False(t, r.evalCondition(nil, "__if_false__", output))
	})

	t.Run("empty condition", func(t *testing.T) {
		assert.True(t, r.evalCondition(nil, "", nil))
	})

	t.Run("literal false condition", func(t *testing.T) {
		assert.False(t, r.evalCondition(nil, "false", nil))
	})

	t.Run("literal zero condition", func(t *testing.T) {
		assert.False(t, r.evalCondition(nil, "0", nil))
	})

	t.Run("non-empty condition", func(t *testing.T) {
		assert.True(t, r.evalCondition(nil, "true", nil))
	})
}

func TestResolveGeneratorRefs(t *testing.T) {
	reg := builtin.DefaultRegistry()

	t.Run("email generator", func(t *testing.T) {
		result := resolveGeneratorRefs(`{"email":"${generator.email}"}`, reg, nil)
		assert.NotContains(t, result, "${generator.email}")
		assert.Contains(t, result, "@")
	})

	t.Run("uuid generator", func(t *testing.T) {
		result := resolveGeneratorRefs(`{"id":"${generator.uuid}"}`, reg, nil)
		assert.NotContains(t, result, "${generator.uuid}")
		assert.Contains(t, result, "-")
	})

	t.Run("multiple generators in one string", func(t *testing.T) {
		result := resolveGeneratorRefs(`${generator.email},${generator.uuid}`, reg, nil)
		assert.NotContains(t, result, "${generator.")
		assert.Contains(t, result, ",")
		assert.Contains(t, result, "@")
	})

	t.Run("unknown generator preserved", func(t *testing.T) {
		result := resolveGeneratorRefs(`${generator.nonexistent}`, reg, nil)
		assert.Contains(t, result, "${generator.nonexistent}")
	})

	t.Run("empty string returns empty", func(t *testing.T) {
		result := resolveGeneratorRefs("", reg, nil)
		assert.Equal(t, "", result)
	})

	t.Run("nil registry returns original", func(t *testing.T) {
		result := resolveGeneratorRefs(`${generator.email}`, nil, nil)
		assert.Equal(t, `${generator.email}`, result)
	})

	t.Run("mixed with literal text", func(t *testing.T) {
		result := resolveGeneratorRefs(`{"name":"perf_test_user","email":"${generator.email}"}`, reg, nil)
		assert.Contains(t, result, `"name":"perf_test_user"`)
		assert.NotContains(t, result, "${generator.email}")
		assert.Contains(t, result, "@")
	})
}
