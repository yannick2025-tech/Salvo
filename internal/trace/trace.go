// Package trace provides distributed tracing for the Salvo test engine.
//
// A Trace captures the full execution history of a single DAG run. Each
// node execution is recorded as a Span containing timing, status, and
// I/O metadata. Traces are written to an in-memory ring buffer and
// optionally persisted to SQLite for long-term analysis.
//
// Usage:
//
//	tracer := trace.NewTracer(trace.Config{BufferSize: 1000})
//
//	// Start a trace for a scene run.
//	tctx := tracer.Start(ctx, sceneID, runID)
//
//	// Spans are recorded automatically by the instrumented executor,
//	// or manually:
//	span := tctx.StartSpan("node-1")
//	// ... do work ...
//	span.Finish(output, err)
//
//	// Close the trace when the run completes.
//	tctx.Finish()
package trace

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// BroadcastFunc is called after a span is recorded to broadcast the
// event to external subscribers (e.g. via WebSocket).
type BroadcastFunc func(runID string, chainID string, nodeID string, status string, durationNs int64, errMsg string, loopIndex int)

// SpanStatus represents the outcome of a span execution.
type SpanStatus string

const (
	SpanStatusOK       SpanStatus = "ok"
	SpanStatusError    SpanStatus = "error"
	SpanStatusSkip     SpanStatus = "skip"
	SpanStatusCanceled SpanStatus = "canceled"
)

// Span records the execution details of a single DAG node.
type Span struct {
	ID           snowflake.ID `json:"id"`
	TraceID      snowflake.ID `json:"trace_id"`
	ChainID      string       `json:"chain_id"`
	NodeID       string       `json:"node_id"`
	ParentNodeID string       `json:"parent_node_id,omitempty"`
	Status       SpanStatus   `json:"status"`
	Error        string       `json:"error,omitempty"`
	Input        string       `json:"input,omitempty"`
	Output       string       `json:"output,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	Duration     time.Duration `json:"duration"`
}

// Trace records the full execution history of a single DAG run.
type Trace struct {
	ID        snowflake.ID `json:"id"`
	SceneID   snowflake.ID `json:"scene_id"`
	RunID     snowflake.ID `json:"run_id"`
	Status    SpanStatus   `json:"status"`
	Error     string       `json:"error,omitempty"`
	Spans     []*Span      `json:"spans"`
	StartedAt time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Duration  time.Duration `json:"duration"`

	mu sync.Mutex
}

// NodeStats holds cumulative execution counters for one (chain_id, node_id)
// pair within a trace. Pass/Fail/Skip count finished iterations and only
// grow; RunningIdx tracks in-flight iteration indexes; LastIndex is the
// highest iteration index already applied (for idempotent dedup).
type NodeStats struct {
	Pass       int
	Fail       int
	Skip       int
	RunningIdx map[int]struct{} // in-flight loop indexes
	LastIndex  int              // highest applied iteration index; -1 = none
}

// applyTerminal records a finished iteration idempotently: iterations at or
// below LastIndex have already been counted (e.g. the per-iteration event
// followed by the span-level Finish broadcast) and are skipped.
func (ns *NodeStats) applyTerminal(counter *int, loopIndex int) {
	if loopIndex <= ns.LastIndex {
		return
	}
	*counter++
	delete(ns.RunningIdx, loopIndex)
	ns.LastIndex = loopIndex
}

// clearRunning drops all in-flight iteration markers. Called when the trace
// finishes so subscribers never see stale running counts after a run ends.
func (ns *NodeStats) clearRunning() {
	ns.RunningIdx = make(map[int]struct{})
}

// RunStats holds the cumulative per-node counters of one run. The runner
// spawns chains continuously (each chain executes the DAG once and gets its
// own Trace), so counters must aggregate at the run level — a per-Trace
// snapshot would only cover the most recently started chain.
type RunStats struct {
	chains map[string]map[string]*NodeStats // chainID → nodeID → counters
	order  []string                         // insertion order of real chains (FIFO merge eviction)
}

const (
	// maxTrackedRuns bounds how many finished runs keep their stats in
	// memory for late subscribers (page re-entry after the run ended).
	maxTrackedRuns = 32
	// maxChainsPerRun bounds per-chain detail for long duration runs.
	// Older chains are folded into mergedChainID so run-level totals stay
	// exact while memory stays bounded.
	maxChainsPerRun = 2048
	// mergedChainID is the pseudo chain that accumulates evicted chains.
	mergedChainID = "_merged"
)

// mergeOldestChain folds the oldest tracked chain into the merged pseudo
// chain. Per-chain detail is lost for the evicted chain but run-level
// totals (what the aggregate view shows) stay exact. In-flight markers of
// the evicted chain may collide with the merged set (iteration indexes are
// per-chain), so a still-running evicted chain can lose a running marker —
// acceptable because eviction only targets the oldest chains, which are
// almost always finished.
func (rs *RunStats) mergeOldestChain() {
	if len(rs.order) <= maxChainsPerRun {
		return
	}
	oldest := rs.order[0]
	rs.order = rs.order[1:]
	src := rs.chains[oldest]
	delete(rs.chains, oldest)
	if src == nil {
		return
	}
	dst, ok := rs.chains[mergedChainID]
	if !ok {
		dst = make(map[string]*NodeStats)
		rs.chains[mergedChainID] = dst
	}
	for nodeID, ns := range src {
		d := dst[nodeID]
		if d == nil {
			d = &NodeStats{RunningIdx: make(map[int]struct{}), LastIndex: -1}
			dst[nodeID] = d
		}
		d.Pass += ns.Pass
		d.Fail += ns.Fail
		d.Skip += ns.Skip
		if ns.LastIndex > d.LastIndex {
			d.LastIndex = ns.LastIndex
		}
		for idx := range ns.RunningIdx {
			d.RunningIdx[idx] = struct{}{}
		}
	}
}

// AddSpan appends a span to the trace.
func (t *Trace) AddSpan(s *Span) {
	t.mu.Lock()
	t.Spans = append(t.Spans, s)
	t.mu.Unlock()
}

// SnapshotSpans returns a copy of the current spans. Readers that may run
// concurrently with an in-flight run (e.g. the WebSocket subscribe snapshot
// or the trace REST API) must use this instead of reading t.Spans directly:
// for an active run the trace is being mutated by AddSpan.
func (t *Trace) SnapshotSpans() []*Span {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*Span, len(t.Spans))
	copy(out, t.Spans)
	return out
}

// Context is a handle for recording spans within a single trace.
// It is created by Tracer.Start and must be closed with Finish
// when the DAG run completes.
type Context struct {
	trace  *Trace
	tracer *Tracer
	node   *snowflake.Node

	// finished guards Finish against double invocation. Callers may
	// finish a trace explicitly (e.g. FinishWithError on failure) and
	// again via defer; only the first call records the trace.
	finished atomic.Bool
}

// TraceID returns the ID of the underlying trace.
func (c *Context) TraceID() snowflake.ID {
	return c.trace.ID
}

// SceneID returns the scene ID of the underlying trace.
func (c *Context) SceneID() snowflake.ID {
	return c.trace.SceneID
}

// StartSpan begins a new span for the given node ID.
// Call Finish on the returned span when the node execution completes.
func (c *Context) StartSpan(nodeID string) *SpanBuilder {
	return &SpanBuilder{
		span: &Span{
			ID:        c.node.Generate(),
			TraceID:   c.trace.ID,
			NodeID:    nodeID,
			StartedAt: time.Now().UTC(),
		},
		ctx: c,
	}
}

// Finish marks the trace as completed with the given status.
// It is idempotent: only the first call takes effect, so an explicit
// finish followed by a deferred finish records the trace exactly once.
func (c *Context) Finish() {
	if !c.finished.CompareAndSwap(false, true) {
		return
	}

	c.trace.FinishedAt = time.Now().UTC()
	c.trace.Duration = c.trace.FinishedAt.Sub(c.trace.StartedAt)

	if c.trace.Status == "" {
		c.trace.Status = SpanStatusOK
	}

	// Aggregate: when the trace still carries the default OK status (no
	// explicit FinishWithError/FinishWithCanceled was called), any error
	// span — e.g. a soft-failed node — marks the whole trace as error so
	// failed chains surface directly in the trace list.
	if c.trace.Status == SpanStatusOK {
		for _, s := range c.trace.Spans {
			if s.Status == SpanStatusError {
				c.trace.Status = SpanStatusError
				c.trace.Error = fmt.Sprintf("span %s failed: %s", s.NodeID, s.Error)
				break
			}
		}
	}

	// Running markers of in-flight iterations are cleared at the run level
	// (Tracer.FinishRun, called by the runner when the whole run completes);
	// per-node terminal broadcasts already clear their own markers.
	c.tracer.record(c.trace)
}

// FinishWithError marks the trace as failed with an error message.
func (c *Context) FinishWithError(err string) {
	c.trace.Status = SpanStatusError
	c.trace.Error = err
	c.Finish()
}

// FinishWithCanceled marks the trace as canceled (manual stop) with a
// reason message. Canceled traces are displayed as warnings, not errors.
func (c *Context) FinishWithCanceled(reason string) {
	c.trace.Status = SpanStatusCanceled
	c.trace.Error = reason
	c.Finish()
}

// SpanBuilder is a fluent builder for recording span details.
type SpanBuilder struct {
	span *Span
	ctx  *Context

	// lastIterIndex is the highest loop index broadcast via Running; the
	// span-level Finish/Skip/FinishCanceled broadcasts carry it so the
	// frontend can dedup them against per-iteration events.
	lastIterIndex int
}

// SetInput records a summary of the span input.
func (b *SpanBuilder) SetInput(s string) *SpanBuilder {
	b.span.Input = s
	return b
}

// SetChainID sets the chain ID for this span.
func (b *SpanBuilder) SetChainID(chainID string) *SpanBuilder {
	b.span.ChainID = chainID
	return b
}

// SetParentNodeID sets the parent node ID for this span.
func (b *SpanBuilder) SetParentNodeID(parentNodeID string) *SpanBuilder {
	b.span.ParentNodeID = parentNodeID
	return b
}

// Finish completes the span with the given output and error.
func (b *SpanBuilder) Finish(output string, err error) {
	b.span.FinishedAt = time.Now().UTC()
	b.span.Duration = b.span.FinishedAt.Sub(b.span.StartedAt)
	b.span.Output = output

	if err != nil {
		b.span.Status = SpanStatusError
		b.span.Error = err.Error()
	} else {
		b.span.Status = SpanStatusOK
	}

	b.ctx.trace.AddSpan(b.span)
	b.emitBroadcast(string(b.span.Status), b.lastIterIndex)
}

// Skip marks the span as skipped (e.g. conditional edge not taken).
func (b *SpanBuilder) Skip(reason string) {
	b.span.FinishedAt = time.Now().UTC()
	b.span.Duration = b.span.FinishedAt.Sub(b.span.StartedAt)
	b.span.Status = SpanStatusSkip
	b.span.Error = reason
	b.ctx.trace.AddSpan(b.span)
	b.emitBroadcast(string(b.span.Status), b.lastIterIndex)
}

// FinishCanceled completes the span with a "canceled" status, used when
// the scene was manually stopped. The span is displayed as a warning.
func (b *SpanBuilder) FinishCanceled(output string, err error) {
	b.span.FinishedAt = time.Now().UTC()
	b.span.Duration = b.span.FinishedAt.Sub(b.span.StartedAt)
	b.span.Output = output
	b.span.Status = SpanStatusCanceled
	if err != nil {
		b.span.Error = err.Error()
	}
	b.ctx.trace.AddSpan(b.span)
	b.emitBroadcast(string(b.span.Status), b.lastIterIndex)
}

// BroadcastRunning emits a "running" event for a span that is about to
// start a loop iteration. This allows subscribers to see intermediate
// progress during multi-iteration node execution.
func (b *SpanBuilder) BroadcastRunning(loopIndex int) {
	b.lastIterIndex = loopIndex
	b.emitBroadcast("running", loopIndex)
}

// BroadcastIterationResult emits the outcome of a single loop iteration
// (ok or error) so subscribers can maintain cumulative per-iteration
// counters. The span-level Finish broadcast repeats the final iteration
// with the same loop index and is deduplicated downstream.
func (b *SpanBuilder) BroadcastIterationResult(loopIndex int, failed bool) {
	b.lastIterIndex = loopIndex
	status := "ok"
	if failed {
		status = "error"
	}
	b.emitBroadcast(status, loopIndex)
}

// emitBroadcast updates the run's cumulative stats and calls the tracer's
// broadcast function if set.
func (b *SpanBuilder) emitBroadcast(status string, loopIndex int) {
	b.ctx.tracer.updateRunStats(b.ctx.trace.RunID, b.span.ChainID, b.span.NodeID, status, loopIndex)
	fn := b.ctx.tracer.broadcast
	if fn == nil {
		return
	}
	runID := b.ctx.trace.RunID.String()
	fn(runID, b.span.ChainID, b.span.NodeID, status, int64(b.span.Duration), b.span.Error, loopIndex)
}

// Config holds the configuration for a Tracer.
type Config struct {
	// BufferSize is the maximum number of completed traces kept in
	// the in-memory ring buffer. Older traces are evicted when full.
	BufferSize int

	// Persister is an optional hook for persisting traces to long-term
	// storage (e.g. SQLite). Called after each trace is recorded.
	Persister TracePersister

	// Broadcast is an optional hook called after each span is recorded
	// to push the event to external subscribers (e.g. WebSocket clients).
	Broadcast BroadcastFunc
}

// TracePersister is the interface for persisting traces to long-term storage.
type TracePersister interface {
	SaveTrace(ctx context.Context, tr *Trace) error
}

// Tracer manages trace lifecycle and storage.
type Tracer struct {
	cfg       Config
	buffer    []*Trace
	active    map[snowflake.ID]*Trace // runID → in-flight trace, registered at Start
	mu        sync.RWMutex
	node      *snowflake.Node
	persister TracePersister
	broadcast BroadcastFunc

	// runStats aggregates cumulative per-node counters across every chain
	// of a run (chains are traced individually, so no single Trace holds
	// the full picture). Keyed by runID; bounded by maxTrackedRuns.
	runStats      map[snowflake.ID]*RunStats
	runStatsOrder []snowflake.ID // FIFO eviction order of tracked runs
}

// NewTracer creates a new Tracer with the given configuration.
func NewTracer(cfg Config) (*Tracer, error) {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}

	n, err := snowflake.NewNode(2)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		cfg:       cfg,
		buffer:    make([]*Trace, 0, cfg.BufferSize),
		active:    make(map[snowflake.ID]*Trace),
		node:      n,
		persister: cfg.Persister,
		broadcast: cfg.Broadcast,
		runStats:  make(map[snowflake.ID]*RunStats),
	}, nil
}

// Start creates a new trace for the given scene and run IDs.
// The returned Context must be closed with Finish or FinishWithError.
// The trace is registered in the active map immediately so that ByRunID
// can find it while the run is still in flight (e.g. for the WebSocket
// subscribe snapshot and the live trace REST endpoint).
func (t *Tracer) Start(ctx context.Context, sceneID, runID snowflake.ID) *Context {
	now := time.Now().UTC()
	tr := &Trace{
		ID:        t.node.Generate(),
		SceneID:   sceneID,
		RunID:     runID,
		Status:    SpanStatusOK,
		Spans:     make([]*Span, 0),
		StartedAt: now,
	}

	t.mu.Lock()
	t.active[runID] = tr
	t.mu.Unlock()

	return &Context{
		trace:  tr,
		tracer: t,
		node:   t.node,
	}
}

// record adds a completed trace to the in-memory buffer and persists it.
// When the buffer is full, the oldest trace is evicted.
func (t *Tracer) record(tr *Trace) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.active, tr.RunID)

	if len(t.buffer) >= t.cfg.BufferSize {
		t.buffer = t.buffer[1:]
	}
	t.buffer = append(t.buffer, tr)

	if t.persister != nil {
		go func() {
			_ = t.persister.SaveTrace(context.Background(), tr)
		}()
	}
}

// newRunStatsLocked creates the stats bucket for a run, evicting the oldest
// tracked run once maxTrackedRuns is exceeded. Callers must hold t.mu.
func (t *Tracer) newRunStatsLocked(runID snowflake.ID) *RunStats {
	if len(t.runStatsOrder) >= maxTrackedRuns {
		old := t.runStatsOrder[0]
		t.runStatsOrder = t.runStatsOrder[1:]
		delete(t.runStats, old)
	}
	rs := &RunStats{chains: make(map[string]map[string]*NodeStats)}
	t.runStats[runID] = rs
	t.runStatsOrder = append(t.runStatsOrder, runID)
	return rs
}

// updateRunStats maintains the per-run, per-(chain, node) cumulative
// counters. Terminal statuses are applied idempotently via LastIndex so the
// per-iteration event followed by the span-level Finish broadcast counts
// exactly once.
func (t *Tracer) updateRunStats(runID snowflake.ID, chainID, nodeID, status string, loopIndex int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rs := t.runStats[runID]
	if rs == nil {
		rs = t.newRunStatsLocked(runID)
	}

	chainMap, ok := rs.chains[chainID]
	if !ok {
		chainMap = make(map[string]*NodeStats)
		rs.chains[chainID] = chainMap
		rs.order = append(rs.order, chainID)
		rs.mergeOldestChain()
	}
	ns, ok := chainMap[nodeID]
	if !ok {
		ns = &NodeStats{RunningIdx: make(map[int]struct{}), LastIndex: -1}
		chainMap[nodeID] = ns
	}

	switch status {
	case "running":
		if loopIndex > ns.LastIndex {
			ns.RunningIdx[loopIndex] = struct{}{}
		}
	case "ok":
		ns.applyTerminal(&ns.Pass, loopIndex)
	case "error":
		ns.applyTerminal(&ns.Fail, loopIndex)
	case "skip", "canceled":
		// Canceled iterations surface as skip in the UI (warning, not error).
		ns.applyTerminal(&ns.Skip, loopIndex)
	}
}

// SnapshotRunStats returns a deep copy of the cumulative per-node stats of a
// run, aggregated across every chain spawned by it. This is what the
// WebSocket subscribe snapshot sends so a client re-entering the realtime
// page rebuilds the full cumulative state. Safe for concurrent use with an
// in-flight run.
func (t *Tracer) SnapshotRunStats(runID snowflake.ID) map[string]map[string]*NodeStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	rs := t.runStats[runID]
	if rs == nil {
		return nil
	}
	out := make(map[string]map[string]*NodeStats, len(rs.chains))
	for chainID, nodes := range rs.chains {
		m := make(map[string]*NodeStats, len(nodes))
		for nodeID, ns := range nodes {
			cp := *ns
			cp.RunningIdx = make(map[int]struct{}, len(ns.RunningIdx))
			for k := range ns.RunningIdx {
				cp.RunningIdx[k] = struct{}{}
			}
			m[nodeID] = &cp
		}
		out[chainID] = m
	}
	return out
}

// FinishRun clears the in-flight iteration markers of every chain in the
// run. Called by the runner when the run completes so late subscribers
// never see stale running counts. The cumulative counters are kept so the
// final totals remain visible after the run ends.
func (t *Tracer) FinishRun(runID snowflake.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rs := t.runStats[runID]
	if rs == nil {
		return
	}
	for _, nodes := range rs.chains {
		for _, ns := range nodes {
			ns.clearRunning()
		}
	}
}

// LoadFromDB loads historical traces from the persistence store into memory.
// Called on startup to restore data after process restart.
func (t *Tracer) LoadFromDB(ctx context.Context) error {
	if t.persister == nil {
		return nil
	}

	loader, ok := t.persister.(interface {
		ListAllTraces(context.Context, int) ([]*Trace, error)
	})
	if !ok {
		return nil
	}

	traces, err := loader.ListAllTraces(ctx, t.cfg.BufferSize)
	if err != nil {
		return fmt.Errorf("tracer: load from db: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, tr := range traces {
		if len(t.buffer) >= t.cfg.BufferSize {
			break
		}
		t.buffer = append(t.buffer, tr)
	}

	return nil
}

// Get retrieves a trace by ID from the in-memory buffer.
func (t *Tracer) Get(id snowflake.ID) (*Trace, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, tr := range t.buffer {
		if tr.ID == id {
			return tr, true
		}
	}
	return nil, false
}

// List returns traces from the in-memory buffer, most recent first.
// The limit parameter caps the number of results; 0 means use default.
func (t *Tracer) List(limit int, offset int) []*Trace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	n := len(t.buffer)
	start := n - 1 - offset
	if start < 0 {
		return nil
	}
	end := start - limit
	if end < -1 {
		end = -1
	}

	result := make([]*Trace, 0, limit)
	for i := start; i > end; i-- {
		result = append(result, t.buffer[i])
	}
	return result
}

func (t *Tracer) Total() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.buffer)
}

// ListByScene returns traces for a specific scene ID, most recent first.
func (t *Tracer) ListByScene(sceneID snowflake.ID, limit int, offset int) []*Trace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var all []*Trace
	for i := len(t.buffer) - 1; i >= 0; i-- {
		if t.buffer[i].SceneID == sceneID {
			all = append(all, t.buffer[i])
		}
	}

	if offset >= len(all) {
		return nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}

// ByRunID returns a trace for a specific run ID. It finds in-flight traces
// first (registered at Start, before any span is recorded) and falls back
// to the completed-trace buffer. This lets live viewers (WebSocket
// subscribe snapshot, trace REST API) see the run while it executes.
func (t *Tracer) ByRunID(runID snowflake.ID) (*Trace, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if tr, ok := t.active[runID]; ok {
		return tr, true
	}

	for _, tr := range t.buffer {
		if tr.RunID == runID {
			return tr, true
		}
	}
	return nil, false
}
