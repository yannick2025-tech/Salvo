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
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// SpanStatus represents the outcome of a span execution.
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "ok"
	SpanStatusError SpanStatus = "error"
	SpanStatusSkip  SpanStatus = "skip"
)

// Span records the execution details of a single DAG node.
type Span struct {
	ID        snowflake.ID `json:"id"`
	TraceID   snowflake.ID `json:"trace_id"`
	NodeID    string       `json:"node_id"`
	Status    SpanStatus   `json:"status"`
	Error     string       `json:"error,omitempty"`
	Input     string       `json:"input,omitempty"`
	Output    string       `json:"output,omitempty"`
	StartedAt time.Time    `json:"started_at"`
	FinishedAt time.Time   `json:"finished_at"`
	Duration  time.Duration `json:"duration"`
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
	FinishedAt time.Time   `json:"finished_at"`
	Duration  time.Duration `json:"duration"`
	mu        sync.Mutex
}

// AddSpan appends a span to the trace.
func (t *Trace) AddSpan(s *Span) {
	t.mu.Lock()
	t.Spans = append(t.Spans, s)
	t.mu.Unlock()
}

// Context is a handle for recording spans within a single trace.
// It is created by Tracer.Start and must be closed with Finish
// when the DAG run completes.
type Context struct {
	trace  *Trace
	tracer *Tracer
	node   *snowflake.Node
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
func (c *Context) Finish() {
	c.trace.FinishedAt = time.Now().UTC()
	c.trace.Duration = c.trace.FinishedAt.Sub(c.trace.StartedAt)

	if c.trace.Status == "" {
		c.trace.Status = SpanStatusOK
	}

	c.tracer.record(c.trace)
}

// FinishWithError marks the trace as failed with an error message.
func (c *Context) FinishWithError(err string) {
	c.trace.Status = SpanStatusError
	c.trace.Error = err
	c.Finish()
}

// SpanBuilder is a fluent builder for recording span details.
type SpanBuilder struct {
	span *Span
	ctx  *Context
}

// SetInput records a summary of the span input.
func (b *SpanBuilder) SetInput(s string) *SpanBuilder {
	b.span.Input = s
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
}

// Skip marks the span as skipped (e.g. conditional edge not taken).
func (b *SpanBuilder) Skip(reason string) {
	b.span.FinishedAt = time.Now().UTC()
	b.span.Duration = b.span.FinishedAt.Sub(b.span.StartedAt)
	b.span.Status = SpanStatusSkip
	b.span.Error = reason
	b.ctx.trace.AddSpan(b.span)
}

// Config holds the configuration for a Tracer.
type Config struct {
	// BufferSize is the maximum number of completed traces kept in
	// the in-memory ring buffer. Older traces are evicted when full.
	BufferSize int
}

// Tracer manages trace lifecycle and storage.
type Tracer struct {
	cfg    Config
	buffer []*Trace
	mu     sync.RWMutex
	node   *snowflake.Node
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
		cfg:    cfg,
		buffer: make([]*Trace, 0, cfg.BufferSize),
		node:   n,
	}, nil
}

// Start creates a new trace for the given scene and run IDs.
// The returned Context must be closed with Finish or FinishWithError.
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

	return &Context{
		trace:  tr,
		tracer: t,
		node:   t.node,
	}
}

// record adds a completed trace to the in-memory buffer.
// When the buffer is full, the oldest trace is evicted.
func (t *Tracer) record(tr *Trace) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.buffer) >= t.cfg.BufferSize {
		t.buffer = t.buffer[1:]
	}
	t.buffer = append(t.buffer, tr)
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
func (t *Tracer) List(limit int) []*Trace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	n := len(t.buffer)
	if n > limit {
		n = limit
	}

	result := make([]*Trace, n)
	for i := 0; i < n; i++ {
		result[i] = t.buffer[len(t.buffer)-1-i]
	}
	return result
}

// ListByScene returns traces for a specific scene ID, most recent first.
func (t *Tracer) ListByScene(sceneID snowflake.ID, limit int) []*Trace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var result []*Trace
	for i := len(t.buffer) - 1; i >= 0 && len(result) < limit; i-- {
		if t.buffer[i].SceneID == sceneID {
			result = append(result, t.buffer[i])
		}
	}
	return result
}

// ByRunID returns a trace for a specific run ID.
func (t *Tracer) ByRunID(runID snowflake.ID) (*Trace, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, tr := range t.buffer {
		if tr.RunID == runID {
			return tr, true
		}
	}
	return nil, false
}
