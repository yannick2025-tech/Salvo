package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/cascade"
	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/lifecycle"
	"github.com/yannick2025-tech/Salvo/internal/core/pool"
	"github.com/yannick2025-tech/Salvo/internal/core/variable"
	"github.com/yannick2025-tech/Salvo/internal/generator"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/plugin"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

type RunMode string

const (
	RunModeCount    RunMode = "count"
	RunModeDuration RunMode = "duration"
)

type Config struct {
	SceneID    snowflake.ID
	Workers    int
	RunMode    RunMode
	Count      int64
	Duration   time.Duration
	Timeout    time.Duration
	Variables  map[string]string
}

func (c Config) Validate() error {
	if c.SceneID == 0 {
		return fmt.Errorf("runner: scene_id is required")
	}
	if c.Workers <= 0 {
		return fmt.Errorf("runner: workers must be > 0, got %d", c.Workers)
	}
	switch c.RunMode {
	case RunModeCount:
		if c.Count <= 0 {
			return fmt.Errorf("runner: count must be > 0, got %d", c.Count)
		}
	case RunModeDuration:
		if c.Duration <= 0 {
			return fmt.Errorf("runner: duration must be > 0, got %s", c.Duration)
		}
	default:
		return fmt.Errorf("runner: invalid run_mode %q", c.RunMode)
	}
	return nil
}

type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type Stats struct {
	TotalReqs   atomic.Int64
	SuccessReqs atomic.Int64
	FailedReqs  atomic.Int64
	latencies   sync.Mutex
	latencyList []time.Duration
}

func (s *Stats) RecordLatency(d time.Duration, success bool) {
	s.TotalReqs.Add(1)
	if success {
		s.SuccessReqs.Add(1)
	} else {
		s.FailedReqs.Add(1)
	}
	s.latencies.Lock()
	s.latencyList = append(s.latencyList, d)
	s.latencies.Unlock()
}

func (s *Stats) LatencyPercentiles() (avg, p50, p95, p99 time.Duration) {
	s.latencies.Lock()
	list := make([]time.Duration, len(s.latencyList))
	copy(list, s.latencyList)
	s.latencies.Unlock()

	if len(list) == 0 {
		return 0, 0, 0, 0
	}

	var total time.Duration
	for _, l := range list {
		total += l
	}
	avg = total / time.Duration(len(list))

	p50 = percentile(list, 50)
	p95 = percentile(list, 95)
	p99 = percentile(list, 99)
	return
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

type Runner struct {
	cfg      Config
	status   atomic.Value
	stats    *Stats
	cancel   context.CancelFunc
	ctx      context.Context
	mu       sync.Mutex
	done     chan struct{}

	scenes  repo.SceneRepo
	nodes   repo.NodeRepo
	edges   repo.EdgeRepo
	runs    repo.RunRecordRepo
	tracer  *tracelib.Tracer
	runID   snowflake.ID
	nodeGen *snowflake.Node

	startedAt  time.Time
	finishedAt time.Time
}

func New(cfg Config, scenes repo.SceneRepo, nodes repo.NodeRepo, edges repo.EdgeRepo, runs repo.RunRecordRepo, tracer *tracelib.Tracer) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	n, err := snowflake.NewNode(3)
	if err != nil {
		return nil, fmt.Errorf("runner: create snowflake node: %w", err)
	}

	r := &Runner{
		cfg:     cfg,
		stats:   &Stats{},
		scenes:  scenes,
		nodes:   nodes,
		edges:   edges,
		runs:    runs,
		tracer:  tracer,
		nodeGen: n,
		done:    make(chan struct{}),
	}
	r.status.Store(StatusPending)
	return r, nil
}

func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.status.Load() == StatusRunning {
		r.mu.Unlock()
		return fmt.Errorf("runner: already running")
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.status.Store(StatusRunning)
	r.startedAt = time.Now().UTC()
	r.runID = r.nodeGen.Generate()
	r.mu.Unlock()

	defer func() {
		r.finishedAt = time.Now().UTC()
		r.status.Store(StatusDone)
		close(r.done)
	}()

	scene, err := r.scenes.GetByID(r.ctx, r.cfg.SceneID)
	if err != nil {
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: load scene: %w", err)
	}

	dagObj, err := r.buildDAG(scene)
	if err != nil {
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: build dag: %w", err)
	}

	scope, err := r.buildScope(scene)
	if err != nil {
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: build scope: %w", err)
	}

	lc := lifecycle.New()
	lc.Register(lifecycle.HookSceneSetup, func(ctx context.Context) error {
		return r.scenes.Update(ctx, &model.Scene{
			Model:  scene.Model,
			Name:   scene.Name,
			Status: model.SceneStatusRunning,
		})
	})
	lc.Register(lifecycle.HookSceneTeardown, func(ctx context.Context) error {
		return r.scenes.Update(ctx, &model.Scene{
			Model:  scene.Model,
			Name:   scene.Name,
			Status: model.SceneStatusCompleted,
		})
	})

	if err := lc.Run(r.ctx, lifecycle.HookSceneSetup); err != nil {
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: scene setup: %w", err)
	}

	runRecord := &model.RunRecord{
		Model:       model.Model{ID: r.runID},
		SceneID:     r.cfg.SceneID,
		Status:      model.RunStatusRunning,
		WorkerCount: r.cfg.Workers,
		RunMode:     string(r.cfg.RunMode),
		StartedAt:   &r.startedAt,
	}
	if err := r.runs.Create(r.ctx, runRecord); err != nil {
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: create run record: %w", err)
	}

	err = r.execute(dagObj, scope)

	r.finishedAt = time.Now().UTC()
	runRecord.Status = model.RunStatusCompleted
	runRecord.FinishedAt = &r.finishedAt
	runRecord.Duration = r.finishedAt.Sub(r.startedAt).Seconds()
	runRecord.TotalReqs = r.stats.TotalReqs.Load()
	runRecord.SuccessReqs = r.stats.SuccessReqs.Load()
	runRecord.FailedReqs = r.stats.FailedReqs.Load()

	avg, p50, p95, p99 := r.stats.LatencyPercentiles()
	runRecord.AvgLatency = avg.Seconds()
	runRecord.P50Latency = p50.Seconds()
	runRecord.P95Latency = p95.Seconds()
	runRecord.P99Latency = p99.Seconds()

	if err != nil {
		runRecord.Status = model.RunStatusFailed
		runRecord.ErrorMsg = err.Error()
		r.status.Store(StatusFailed)
	} else if r.ctx.Err() != nil {
		runRecord.Status = model.RunStatusCancelled
		r.status.Store(StatusCanceled)
	}

	_ = r.runs.Update(r.ctx, runRecord)
	_ = lc.Run(context.Background(), lifecycle.HookSceneTeardown)

	return err
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *Runner) Status() Status {
	return r.status.Load().(Status)
}

func (r *Runner) Stats() *Stats {
	return r.stats
}

func (r *Runner) RunID() snowflake.ID {
	return r.runID
}

func (r *Runner) Workers() int {
	return r.cfg.Workers
}

func (r *Runner) Done() <-chan struct{} {
	return r.done
}

func (r *Runner) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finishedAt.IsZero() {
		return time.Since(r.startedAt)
	}
	return r.finishedAt.Sub(r.startedAt)
}

func (r *Runner) execute(dagObj *dag.DAG, scope *variable.Scope) error {
	poolCfg := pool.Config{
		RunMode:  pool.RunModeCount,
		Count:    r.cfg.Count,
		Duration: r.cfg.Duration,
	}
	if r.cfg.RunMode == RunModeDuration {
		poolCfg.RunMode = pool.RunModeDuration
	}

	p, err := pool.NewWithContext(r.ctx, r.cfg.Workers, poolCfg)
	if err != nil {
		return fmt.Errorf("runner: create pool: %w", err)
	}

	genRegistry := generator.NewRegistry()
	httpProto := httpprotocol.NewProtocol()
	pluginReg := plugin.NewRegistry()

	task := func(ctx context.Context) error {
		sceneTimeout := r.cfg.Timeout
		if sceneTimeout <= 0 {
			sceneTimeout = 30 * time.Second
		}
		taskCtx, cancel := cascade.NewContext(ctx, r.cfg.SceneID.String(), sceneTimeout)
		defer cancel()

		var execOpts []dag.ExecutorOption
		if r.tracer != nil {
			hook := &dag.HookAdapter{Tracer: r.tracer}
			execOpts = append(execOpts, dag.WithTraceHook(hook, r.cfg.SceneID, r.runID))
		}

		exec := dag.NewExecutor(dagObj, execOpts...)

		resolvedVars := variable.ResolveAll(scope)
		input := &dag.Input{Variables: resolvedVars}

		_ = genRegistry
		_ = httpProto
		_ = pluginReg
		_ = input

		out, err := exec.ExecuteWithTrace(taskCtx)
		if err != nil {
			r.stats.RecordLatency(0, false)
			return err
		}

		if out != nil && out.Response != nil {
			if resp, ok := out.Response.(*httpprotocol.HTTPResponse); ok {
				success := resp.IsSuccess()
				r.stats.RecordLatency(resp.Latency, success)
			} else {
				r.stats.RecordLatency(0, true)
			}
		} else {
			r.stats.RecordLatency(0, true)
		}

		return nil
	}

	switch r.cfg.RunMode {
	case RunModeCount:
		for i := int64(0); i < r.cfg.Count; i++ {
			p.Submit(task)
		}
	case RunModeDuration:
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
					p.Submit(task)
				}
			}
		}()
	}

	return p.Wait()
}

func (r *Runner) buildDAG(scene *model.Scene) (*dag.DAG, error) {
	dagObj := dag.New()

	nodeList, err := r.nodes.List(r.ctx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	nodeMap := make(map[string]snowflake.ID)
	for _, n := range nodeList {
		nodeIDStr := n.ID.String()
		nodeMap[nodeIDStr] = n.ID

		dagNode, buildErr := r.buildDAGNode(n)
		if buildErr != nil {
			return nil, buildErr
		}
		if addErr := dagObj.AddNode(dagNode); addErr != nil {
			return nil, fmt.Errorf("add node %s: %w", nodeIDStr, addErr)
		}
	}

	edgeList, err := r.edges.List(r.ctx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}

	for _, e := range edgeList {
		fromStr := e.FromNode.String()
		toStr := e.ToNode.String()
		edgeType := dag.EdgeNormal
		if e.Condition != "" {
			edgeType = dag.EdgeCondition
		}
		if addErr := dagObj.AddEdge(fromStr, toStr, edgeType, e.Condition); addErr != nil {
			return nil, fmt.Errorf("add edge %s->%s: %w", fromStr, toStr, addErr)
		}
	}

	return dagObj, nil
}

type sceneNode struct {
	id        string
	nodeType  string
	config    string
	timeout   time.Duration
	loopCount int
	mode      dag.ExecMode
}

func (n *sceneNode) ID() string            { return n.id }
func (n *sceneNode) Timeout() time.Duration { return n.timeout }
func (n *sceneNode) LoopCount() int         { return n.loopCount }
func (n *sceneNode) Mode() dag.ExecMode     { return n.mode }

func (n *sceneNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
	switch n.nodeType {
	case model.NodeTypeHTTP:
		return n.executeHTTP(ctx, input)
	case model.NodeTypeDelay:
		return n.executeDelay(ctx)
	case model.NodeTypeCondition:
		return n.executeCondition(input)
	default:
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": n.nodeType}}, nil
	}
}

func (n *sceneNode) executeHTTP(ctx context.Context, input *dag.Input) (*dag.Output, error) {
	var cfg struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
		Timeout float64           `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		return nil, fmt.Errorf("parse http config: %w", err)
	}

	url := cfg.URL
	if input != nil && input.Variables != nil {
		scope := variable.NewScope()
		for k, v := range input.Variables {
			scope.Set(k, v)
		}
		resolved, err := variable.ResolveString(scope, url)
		if err == nil {
			url = resolved
		}
	}

	method := cfg.Method
	if method == "" {
		method = "GET"
	}

	timeout := n.timeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout * float64(time.Second))
	}

	req := &httpprotocol.HTTPRequest{
		Method:  httpprotocol.Method(method),
		URL:     url,
		Headers: cfg.Headers,
		Timeout: timeout,
	}
	if cfg.Body != "" {
		req.Body = []byte(cfg.Body)
	}

	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(ctx, req)
	if err != nil {
		return &dag.Output{Error: err}, nil
	}

	return &dag.Output{Response: resp}, nil
}

func (n *sceneNode) executeDelay(ctx context.Context) (*dag.Output, error) {
	var cfg struct {
		DurationMs int64 `json:"duration_ms"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		return nil, fmt.Errorf("parse delay config: %w", err)
	}

	dur := time.Duration(cfg.DurationMs) * time.Millisecond
	if dur <= 0 {
		dur = 100 * time.Millisecond
	}

	select {
	case <-time.After(dur):
		return &dag.Output{Response: map[string]any{"delay_ms": cfg.DurationMs}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (n *sceneNode) executeCondition(input *dag.Input) (*dag.Output, error) {
	return &dag.Output{Response: map[string]any{"condition": true}}, nil
}

func (r *Runner) buildDAGNode(n *model.Node) (*sceneNode, error) {
	sn := &sceneNode{
		id:        n.ID.String(),
		nodeType:  n.Type,
		config:    n.Config,
		loopCount: n.LoopCount,
		mode:      dag.ExecSync,
	}

	if n.LoopCount <= 0 {
		sn.loopCount = 1
	}

	return sn, nil
}

func (r *Runner) buildScope(scene *model.Scene) (*variable.Scope, error) {
	globalScope := variable.NewScope(variable.WithLevel(variable.ScopeGlobal))

	if scene.Variables != "" {
		var vars map[string]string
		if err := json.Unmarshal([]byte(scene.Variables), &vars); err != nil {
			return nil, fmt.Errorf("parse scene variables: %w", err)
		}
		for k, v := range vars {
			globalScope.Set(k, v)
		}
	}

	for k, v := range r.cfg.Variables {
		globalScope.Set(k, v)
	}

	sceneScope := variable.NewScope(
		variable.WithLevel(variable.ScopeScene),
		variable.WithParent(globalScope),
	)

	return sceneScope, nil
}
