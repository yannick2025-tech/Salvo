package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// --- Stats.RecordCanceled ---

func TestStatsRecordCanceled(t *testing.T) {
	s := &Stats{}
	s.RecordCanceled()
	s.RecordCanceled()
	assert.Equal(t, int64(2), s.CanceledReqs.Load())
	// RecordCanceled should NOT increment TotalReqs
	assert.Equal(t, int64(0), s.TotalReqs.Load())
}

// --- Stats.GetAllLatencies ---

func TestStatsGetAllLatencies(t *testing.T) {
	s := &Stats{}
	s.RecordLatency(10*time.Millisecond, true)
	s.RecordLatency(20*time.Millisecond, true)
	s.RecordLatency(30*time.Millisecond, false)

	list := s.GetAllLatencies()
	assert.Len(t, list, 3)
	assert.Equal(t, 10*time.Millisecond, list[0])
	assert.Equal(t, 20*time.Millisecond, list[1])
	assert.Equal(t, 30*time.Millisecond, list[2])
}

func TestStatsGetAllLatenciesEmpty(t *testing.T) {
	s := &Stats{}
	list := s.GetAllLatencies()
	assert.Empty(t, list)
}

// --- percentile ---

func TestPercentileEmpty(t *testing.T) {
	assert.Equal(t, time.Duration(0), percentile(nil, 50))
}

func TestPercentileSingle(t *testing.T) {
	list := []time.Duration{5 * time.Millisecond}
	assert.Equal(t, 5*time.Millisecond, percentile(list, 50))
	assert.Equal(t, 5*time.Millisecond, percentile(list, 99))
}

func TestPercentileMulti(t *testing.T) {
	list := []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// p50 should be around index 5
	p50 := percentile(list, 50)
	assert.Equal(t, 6*time.Duration(1), p50) // idx = 5*10/100 = 0 -> 5... wait: p=50, len=10, idx=50*10/100=5
	p99 := percentile(list, 99)
	assert.Equal(t, 10*time.Duration(1), p99)
}

func TestPercentileOutOfBounds(t *testing.T) {
	list := []time.Duration{1, 2, 3}
	// p=200 -> idx=200*3/100=6 -> capped to 2
	assert.Equal(t, 3*time.Duration(1), percentile(list, 200))
}

// --- RowIterator.Current ---

func TestRowIteratorCurrent(t *testing.T) {
	rows := []map[string]string{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	}
	it := NewRowIterator(rows)
	// index=0 -> Current() idx = -1 -> fallback to rows[0]
	current := it.Current()
	assert.Equal(t, "Alice", current["name"])

	// After Next(), index becomes 1, Current returns rows[0]
	_ = it.Next()
	current = it.Current()
	assert.Equal(t, "Alice", current["name"])
}

func TestRowIteratorCurrentEmpty(t *testing.T) {
	it := NewRowIterator(nil)
	current := it.Current()
	assert.Empty(t, current)
}

// --- RowIterator.RowCount ---

func TestRowIteratorRowCount(t *testing.T) {
	rows := []map[string]string{
		{"a": "1"},
		{"a": "2"},
		{"a": "3"},
	}
	it := NewRowIterator(rows)
	assert.Equal(t, 3, it.RowCount())

	it2 := NewRowIterator(nil)
	assert.Equal(t, 0, it2.RowCount())
}

// --- manager.SetExprRegistry ---

func TestManagerSetExprRegistry(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// SetExprRegistry should not panic
	m.SetExprRegistry(nil)
}

// --- manager.Start (nil deps should not panic) ---

func TestManagerStartWithNilDeps(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// Start with a config that fails validation should return error
	_, err := m.Start(context.Background(), Config{SceneID: 1, RunMode: "invalid"})
	assert.Error(t, err)
}

// --- Runner.GlobalSnapshot ---

func TestRunnerGlobalSnapshotEmpty(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	r := &Runner{
		stats:    &Stats{},
		startedAt: time.Now().Add(-1 * time.Second),
		log:       log,
	}
	snap := r.GlobalSnapshot()
	require.NotNil(t, snap)
	assert.Equal(t, int64(0), snap.TotalRequests)
	assert.Equal(t, int64(0), snap.SuccessCount)
	assert.Equal(t, int64(0), snap.FailCount)
}

func TestRunnerGlobalSnapshotWithData(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	r := &Runner{
		stats:    &Stats{},
		startedAt: time.Now().Add(-2 * time.Second),
		log:       log,
	}
	r.stats.RecordLatency(10*time.Millisecond, true)
	r.stats.RecordLatency(20*time.Millisecond, false)

	snap := r.GlobalSnapshot()
	require.NotNil(t, snap)
	assert.Equal(t, int64(2), snap.TotalRequests)
	assert.Equal(t, int64(1), snap.SuccessCount)
	assert.Equal(t, int64(1), snap.FailCount)
	assert.True(t, snap.AvgLatencyMs > 0)
}

// --- Runner.HttpOnlySnapshot ---

func TestRunnerHttpOnlySnapshotEmpty(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	r := &Runner{
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		startedAt:     time.Now().Add(-1 * time.Second),
		log:           log,
	}
	snap := r.HttpOnlySnapshot()
	require.NotNil(t, snap)
	assert.Equal(t, int64(0), snap.TotalRequests)
}

func TestRunnerHttpOnlySnapshotWithData(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	r := &Runner{
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		startedAt:     time.Now().Add(-2 * time.Second),
		log:           log,
	}
	r.httpOnlyStats.RecordLatency(5*time.Millisecond, true)
	r.httpOnlyStats.RecordLatency(15*time.Millisecond, true)

	snap := r.HttpOnlySnapshot()
	require.NotNil(t, snap)
	assert.Equal(t, int64(2), snap.TotalRequests)
	assert.Equal(t, int64(2), snap.SuccessCount)
}

// --- Runner.NodeSnapshots ---

func TestRunnerNodeSnapshotsEmpty(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	r := &Runner{
		stats:     &Stats{},
		nodeStats: make(map[string]*NodeStats),
		startedAt: time.Now().Add(-1 * time.Second),
		log:       log,
	}
	snaps := r.NodeSnapshots()
	assert.Empty(t, snaps)
}

func TestRunnerNodeSnapshotsWithData(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error"})
	ns := NewNodeStats(1000)
	ns.RecordLatency(10*time.Millisecond, true)
	ns.RecordLatency(20*time.Millisecond, false)

	r := &Runner{
		stats:     &Stats{},
		nodeStats: map[string]*NodeStats{"node-1": ns},
		startedAt: time.Now().Add(-2 * time.Second),
		log:       log,
	}
	snaps := r.NodeSnapshots()
	assert.Len(t, snaps, 1)
	snap, ok := snaps["node-1"]
	require.True(t, ok)
	assert.Equal(t, int64(2), snap.TotalRequests)
	assert.Equal(t, int64(1), snap.SuccessCount)
	assert.Equal(t, int64(1), snap.FailCount)
}

// --- isLikelyFilePath ---

func TestIsLikelyFilePath(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"image.png", true},
		{"document.pdf", true},
		{"archive.zip", true},
		{"config.yaml", true},
		{"data.json", true},
		{"/usr/local/bin/go", true},
		{"C:\\Users\\test\\file.txt", true},
		{"${upload_dir}/file.txt", true}, // has .txt extension, so isLikelyFilePath returns true
		{"hello", false},
		{"plain string", false},
		{"no_extension", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.want, isLikelyFilePath(tt.value))
		})
	}
}

// --- isSensitiveHeader ---

func TestIsSensitiveHeader(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"Cookie", true},
		{"Set-Cookie", true},
		{"X-API-Key", true},
		{"X-Auth-Token", true},
		{"X-CSRF-Token", true},
		{"Proxy-Authorization", true},
		{"X-Custom-Token", true},
		{"My-Secret", true},
		{"Api-Key", true},
		{"Content-Type", false},
		{"Accept", false},
		{"User-Agent", false},
		{"Host", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, isSensitiveHeader(tt.key))
		})
	}
}

// --- maskValue ---

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "****"},
		{"1234", "****"},
		{"12345", "****"},
		{"12345678", "****"},
		{"123456789", "1234****6789"},
		{"abcdefghijklmnop", "abcd****mnop"},
		{"", "****"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, maskValue(tt.input))
		})
	}
}

// --- report.go functions ---

func TestCalculateThroughput(t *testing.T) {
	rr := &model.RunRecord{Duration: 10, TotalReqs: 100}
	assert.Equal(t, 10.0, calculateThroughput(rr))

	rr2 := &model.RunRecord{Duration: 0, TotalReqs: 100}
	assert.Equal(t, 0.0, calculateThroughput(rr2))

	rr3 := &model.RunRecord{Duration: 5, TotalReqs: 0}
	assert.Equal(t, 0.0, calculateThroughput(rr3))
}

func TestCalculateNodeAvgQPS(t *testing.T) {
	snap := &NodeSnapshot{TotalReqs: 50}
	assert.Equal(t, 10.0, calculateNodeAvgQPS(snap, 5))

	snap2 := &NodeSnapshot{TotalReqs: 0}
	assert.Equal(t, 0.0, calculateNodeAvgQPS(snap2, 5))

	snap3 := &NodeSnapshot{TotalReqs: 50}
	assert.Equal(t, 0.0, calculateNodeAvgQPS(snap3, 0))
}

func TestGetNodeName(t *testing.T) {
	assert.Equal(t, "node-123", getNodeName("node-123"))
	assert.Equal(t, "", getNodeName(""))
}

func TestRecordsToGlobalSamples(t *testing.T) {
	records := []TimeSeriesRecord{
		{NodeID: "", SampleTime: time.Now(), QPS: 100, TotalRequests: 50, SuccessCount: 45, FailCount: 5},
		{NodeID: "node-1", SampleTime: time.Now(), QPS: 50, TotalRequests: 25},
		{NodeID: "", SampleTime: time.Now(), QPS: 200, TotalRequests: 100, SuccessCount: 90, FailCount: 10},
	}
	samples := recordsToGlobalSamples(records)
	assert.Len(t, samples, 2)
	assert.Equal(t, float64(100), samples[0].QPS)
	assert.Equal(t, float64(200), samples[1].QPS)
}

func TestRecordsToGlobalSamplesEmpty(t *testing.T) {
	samples := recordsToGlobalSamples(nil)
	assert.Empty(t, samples)
}

func TestRecordsToNodeSamples(t *testing.T) {
	records := []TimeSeriesRecord{
		{NodeID: "node-1", SampleTime: time.Now(), QPS: 50, TotalRequests: 25, SuccessCount: 20, FailCount: 5},
		{NodeID: "", SampleTime: time.Now(), QPS: 100, TotalRequests: 50},
		{NodeID: "node-2", SampleTime: time.Now(), QPS: 30, TotalRequests: 15, SuccessCount: 15, FailCount: 0},
		{NodeID: "node-1", SampleTime: time.Now(), QPS: 60, TotalRequests: 30, SuccessCount: 25, FailCount: 5},
	}
	samples := recordsToNodeSamples(records)
	assert.Len(t, samples, 2)
	assert.Len(t, samples["node-1"], 2)
	assert.Len(t, samples["node-2"], 1)
	assert.Equal(t, float64(50), samples["node-1"][0].QPS)
	assert.Equal(t, float64(60), samples["node-1"][1].QPS)
	assert.Equal(t, float64(30), samples["node-2"][0].QPS)
}

func TestRecordsToNodeSamplesEmpty(t *testing.T) {
	samples := recordsToNodeSamples(nil)
	assert.Empty(t, samples)
}

// --- sceneNode accessors ---

func TestSceneNodeAccessors(t *testing.T) {
	n := &sceneNode{
		id:           "node-1",
		name:         "test-node",
		nodeType:     "http",
		timeout:      5 * time.Second,
		loopCount:    3,
		mode:         dag.ExecSync,
		blockOnError: true,
	}
	assert.Equal(t, "node-1", n.ID())
	assert.Equal(t, 5*time.Second, n.Timeout())
	assert.Equal(t, 3, n.LoopCount())
	assert.Equal(t, dag.ExecSync, n.Mode())
	assert.True(t, n.BlockOnError())
}

func TestSceneNodeAccessorsDefaults(t *testing.T) {
	n := &sceneNode{}
	assert.Equal(t, "", n.ID())
	assert.Equal(t, time.Duration(0), n.Timeout())
	assert.Equal(t, 0, n.LoopCount())
	assert.False(t, n.BlockOnError())
}

// --- variablesOrNil ---

func TestVariablesOrNil(t *testing.T) {
	// nil input
	assert.Nil(t, variablesOrNil(nil))

	// input with nil variables
	input := &dag.Input{}
	assert.Nil(t, variablesOrNil(input))

	// input with variables
	input = &dag.Input{Variables: map[string]any{"key": "value"}}
	result := variablesOrNil(input)
	assert.Equal(t, "value", result["key"])
}

// --- NodeStats.RecordCanceled ---

func TestNodeStatsRecordCanceled(t *testing.T) {
	ns := NewNodeStats(1000)
	ns.RecordCanceled()
	ns.RecordCanceled()
	ns.RecordCanceled()
	assert.Equal(t, int64(3), ns.CanceledReqs.Load())
}
