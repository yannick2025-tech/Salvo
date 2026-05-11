// Package runner implements the test execution engine for Salvo scenarios.
// It loads scene configuration from the store, builds a DAG, and runs
// the scenario using a configurable worker pool with count-based or
// duration-based modes.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/cascade"
	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/lifecycle"
	"github.com/yannick2025-tech/Salvo/internal/core/pool"
	"github.com/yannick2025-tech/Salvo/internal/core/variable"
	"github.com/yannick2025-tech/Salvo/internal/generator"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
	"github.com/yannick2025-tech/Salvo/internal/logger"
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
	SceneID   snowflake.ID
	Workers   int
	RunMode   RunMode
	Count     int64
	Duration  time.Duration
	Timeout   time.Duration
	Variables map[string]string
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
	MinLatency  atomic.Int64
	TTFB        atomic.Int64
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

	ns := d.Nanoseconds()

	if success && ns > 0 {
		for {
			old := s.MinLatency.Load()
			if old == 0 || ns < old {
				if s.MinLatency.CompareAndSwap(old, ns) {
					break
				}
				continue
			}
			break
		}

		if s.TTFB.Load() == 0 {
			s.TTFB.CompareAndSwap(0, ns)
		}
	}

	s.latencies.Lock()
	s.latencyList = append(s.latencyList, d)
	s.latencies.Unlock()
}

func (s *Stats) LatencyPercentiles() (avg, p50, p90, p95, p99 time.Duration) {
	s.latencies.Lock()
	list := make([]time.Duration, len(s.latencyList))
	copy(list, s.latencyList)
	s.latencies.Unlock()

	if len(list) == 0 {
		return 0, 0, 0, 0, 0
	}

	var total time.Duration
	for _, l := range list {
		total += l
	}
	avg = total / time.Duration(len(list))

	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })

	p50 = percentile(list, 50)
	p90 = percentile(list, 90)
	p95 = percentile(list, 95)
	p99 = percentile(list, 99)
	return
}

func (s *Stats) GetAllLatencies() []time.Duration {
	s.latencies.Lock()
	defer s.latencies.Unlock()
	list := make([]time.Duration, len(s.latencyList))
	copy(list, s.latencyList)
	return list
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
	cfg    Config
	status atomic.Value
	stats  *Stats
	cancel context.CancelFunc
	ctx    context.Context
	mu     sync.Mutex
	done   chan struct{}
	log    logger.Logger

	scenes  repo.SceneRepo
	nodes   repo.NodeRepo
	edges   repo.EdgeRepo
	runs    repo.RunRecordRepo
	reports repo.ReportRepo
	tracer  *tracelib.Tracer
	runID   snowflake.ID
	nodeGen *snowflake.Node

	startedAt  time.Time
	finishedAt time.Time

	nodeStats map[string]*NodeStats
	collector *TimeSeriesCollector
	tsStore   TimeSeriesStore
}

func New(cfg Config, scenes repo.SceneRepo, nodes repo.NodeRepo, edges repo.EdgeRepo, runs repo.RunRecordRepo, reports repo.ReportRepo, tracer *tracelib.Tracer, tsStore TimeSeriesStore, log logger.Logger) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	n, err := snowflake.NewNode(3)
	if err != nil {
		return nil, fmt.Errorf("runner: create snowflake node: %w", err)
	}

	r := &Runner{
		cfg:       cfg,
		stats:     &Stats{},
		scenes:    scenes,
		nodes:     nodes,
		edges:     edges,
		runs:      runs,
		reports:   reports,
		tracer:    tracer,
		nodeGen:   n,
		runID:     n.Generate(),
		done:      make(chan struct{}),
		log:       log,
		nodeStats: make(map[string]*NodeStats),
		collector: NewTimeSeriesCollector(TimeSeriesConfig{
			SampleInterval:  1 * time.Second,
			FlushInterval:   10 * time.Second,
			MemoryWindowSec: 300,
			MaxNodes:        100,
		}, n.Generate(), tsStore, nil),
		tsStore: tsStore,
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
	r.mu.Unlock()

	traceID := r.runID.String()
	runLog := r.log.With(
		logger.F("trace_id", traceID),
		logger.F("scene_id", r.cfg.SceneID.String()),
		logger.F("run_id", traceID),
	)

	r.collector.SetStatsProvider(r)

	if err := r.collector.Start(r.startedAt); err != nil {
		runLog.Error("failed to start timeseries collector", logger.F("error", err))
	}

	defer func() {
		if stopErr := r.collector.Stop(); stopErr != nil {
			runLog.Error("failed to stop timeseries collector", logger.F("error", stopErr))
		}
		r.finishedAt = time.Now().UTC()
		r.status.Store(StatusDone)
		close(r.done)
	}()

	startFields := []logger.Field{
		logger.F("workers", r.cfg.Workers),
		logger.F("run_mode", r.cfg.RunMode),
	}
	if r.cfg.RunMode == RunModeCount {
		startFields = append(startFields, logger.F("count", r.cfg.Count))
	} else {
		startFields = append(startFields, logger.F("duration", r.cfg.Duration))
	}
	runLog.Info("scene run started", startFields...)

	scene, err := r.scenes.GetByID(r.ctx, r.cfg.SceneID)
	if err != nil {
		r.status.Store(StatusFailed)
		runLog.Error("failed to load scene", logger.F("error", err))
		return fmt.Errorf("runner: load scene: %w", err)
	}
	runLog.Info("scene loaded", logger.F("scene_name", scene.Name))

	dagObj, err := r.buildDAG(scene)
	if err != nil {
		r.status.Store(StatusFailed)
		runLog.Error("failed to build DAG", logger.F("error", err))
		return fmt.Errorf("runner: build dag: %w", err)
	}
	runLog.Info("DAG built", logger.F("nodes", len(dagObj.Nodes())), logger.F("edges", len(dagObj.Edges())))

	scope, err := r.buildScope(scene)
	if err != nil {
		r.status.Store(StatusFailed)
		runLog.Error("failed to build scope", logger.F("error", err))
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
		Duration:    r.cfg.Duration.Seconds(),
		Count:       r.cfg.Count,
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
	runRecord.TotalReqs = r.stats.TotalReqs.Load()
	runRecord.SuccessReqs = r.stats.SuccessReqs.Load()
	runRecord.FailedReqs = r.stats.FailedReqs.Load()

	avg, p50, p90, p95, p99 := r.stats.LatencyPercentiles()
	runRecord.AvgLatency = avg.Seconds()
	runRecord.P50Latency = p50.Seconds()
	runRecord.P90Latency = p90.Seconds()
	runRecord.P95Latency = p95.Seconds()
	runRecord.P99Latency = p99.Seconds()

	totalReqs := r.stats.TotalReqs.Load()
	successReqs := r.stats.SuccessReqs.Load()

	if err != nil && totalReqs == 0 {
		runRecord.Status = model.RunStatusFailed
		runRecord.ErrorMsg = err.Error()
		r.status.Store(StatusFailed)
	} else if totalReqs > 0 {
		rate := float64(successReqs) / float64(totalReqs)
		if rate < 0.95 {
			runRecord.Status = model.RunStatusFailed
			if err != nil {
				runRecord.ErrorMsg = err.Error()
			} else {
				runRecord.ErrorMsg = fmt.Sprintf("success rate %.1f%% below threshold", rate*100)
			}
			r.status.Store(StatusFailed)
		} else {
			runRecord.Status = model.RunStatusCompleted
			r.status.Store(StatusDone)
		}
	} else if r.ctx.Err() != nil {
		runRecord.Status = model.RunStatusCancelled
		r.status.Store(StatusCanceled)
	} else {
		runRecord.Status = model.RunStatusCompleted
		r.status.Store(StatusDone)
	}

	if err := r.runs.Update(r.ctx, runRecord); err != nil {
		runLog.Error("failed to save run record", logger.F("error", err))
	} else {
		runLog.Info("run completed",
			logger.F("status", runRecord.Status),
			logger.F("total_reqs", runRecord.TotalReqs),
			logger.F("success_reqs", runRecord.SuccessReqs),
			logger.F("failed_reqs", runRecord.FailedReqs),
			logger.F("p50", runRecord.P50Latency),
			logger.F("p95", runRecord.P95Latency),
			logger.F("p99", runRecord.P99Latency),
			logger.F("duration_s", runRecord.Duration),
		)
	}

	if err := r.createReport(runRecord); err != nil {
		runLog.Error("failed to create report", logger.F("error", err))
	}

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

func (r *Runner) createReport(runRecord *model.RunRecord) error {
	successRate := float64(0)
	if runRecord.TotalReqs > 0 {
		successRate = float64(runRecord.SuccessReqs) / float64(runRecord.TotalReqs) * 100
	}

	summary := map[string]any{
		"scene_id":      runRecord.SceneID,
		"run_id":        runRecord.ID,
		"status":        runRecord.Status,
		"worker_count":  runRecord.WorkerCount,
		"run_mode":      runRecord.RunMode,
		"duration_s":    runRecord.Duration,
		"total_reqs":    runRecord.TotalReqs,
		"success_reqs":  runRecord.SuccessReqs,
		"failed_reqs":   runRecord.FailedReqs,
		"success_rate":  fmt.Sprintf("%.1f%%", successRate),
		"avg_latency_s": runRecord.AvgLatency,
		"p50_latency_s": runRecord.P50Latency,
		"p90_latency_s": runRecord.P90Latency,
		"p95_latency_s": runRecord.P95Latency,
		"p99_latency_s": runRecord.P99Latency,
		"min_latency_ms": time.Duration(r.stats.MinLatency.Load()).Seconds() * 1000,
		"p50":           fmt.Sprintf("%.1fms", runRecord.P50Latency*1000),
		"p90":           fmt.Sprintf("%.1fms", runRecord.P90Latency*1000),
		"p95":           fmt.Sprintf("%.1fms", runRecord.P95Latency*1000),
		"p99":           fmt.Sprintf("%.1fms", runRecord.P99Latency*1000),
	}
	summaryBytes, _ := json.Marshal(summary)

	collectorData := r.collector.GetCollectedData()

	r.log.Info("creating report with collected data",
		logger.F("global_samples_count", len(collectorData.GlobalSamples)),
		logger.F("node_samples_count", len(collectorData.NodeSamples)),
	)

	dbRecords, dbErr := r.tsStore.QueryByRunID(r.ctx, r.runID)
	if dbErr != nil {
		r.log.Warn("failed to query full time series from db, falling back to in-memory", logger.F("error", dbErr))
	} else {
		r.log.Info("loaded full time series from database",
			logger.F("db_record_count", len(dbRecords)),
		)
	}

	globalTimeSeries := collectorData.GlobalSamples
	nodeTimeSeriesMap := collectorData.NodeSamples
	if len(dbRecords) > 0 {
		globalTimeSeries = recordsToGlobalSamples(dbRecords)
		nodeTimeSeriesMap = recordsToNodeSamples(dbRecords)
		r.log.Info("using db time series for report",
			logger.F("global_ts_count", len(globalTimeSeries)),
			logger.F("node_ts_keys", len(nodeTimeSeriesMap)),
		)
	}

	for nodeID, samples := range nodeTimeSeriesMap {
		r.log.Debug("node time series data",
			logger.F("node_id", nodeID),
			logger.F("sample_count", len(samples)),
		)
	}

	detail := ReportDetail{
		Metadata: ReportMetadata{
			RunID:           r.runID,
			SceneID:         r.cfg.SceneID,
			Status:          string(runRecord.Status),
			StartedAt:       r.startedAt,
			FinishedAt:      r.finishedAt,
			DurationSec:     r.finishedAt.Sub(r.startedAt).Seconds(),
			WorkerCount:     r.cfg.Workers,
			RunMode:         string(r.cfg.RunMode),
			Count:           r.cfg.Count,
			PlannedDuration: r.cfg.Duration.Seconds(),
			PlannedCount:    r.cfg.Count,
			GeneratedAt:     time.Now().UTC(),
			Version:         "1.0",
		},
		GlobalSummary: GlobalSummary{
			TotalRequests: runRecord.TotalReqs,
			SuccessCount:  runRecord.SuccessReqs,
			FailCount:     runRecord.FailedReqs,
			SuccessRate:   successRate,
			AvgLatencyMs:  runRecord.AvgLatency * 1000,
			P50LatencyMs:  runRecord.P50Latency * 1000,
			P90LatencyMs:  runRecord.P90Latency * 1000,
			P95LatencyMs:  runRecord.P95Latency * 1000,
			P99LatencyMs:  runRecord.P99Latency * 1000,
			MinLatencyMs:  time.Duration(r.stats.MinLatency.Load()).Seconds() * 1000,
			TTFB:          time.Duration(r.stats.TTFB.Load()).Seconds() * 1000,
			Throughput:    calculateThroughput(runRecord),
			PeakQPS:       collectorData.GlobalPeakQPS,
		},
		GlobalTimeSeries: globalTimeSeries,
		NodeMetrics:      []NodeMetricDetail{},
		ErrorSummary:     collectorData.ErrorItems,
	}

	nodeNameMap := make(map[string]string)
	if nodeList, err := r.nodes.List(r.ctx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000}); err == nil {
		for _, n := range nodeList {
			nodeNameMap[n.ID.String()] = n.Name
		}
	}

	aggErrorCodes := make(map[string]int64)
	for nodeID, nodeStat := range r.nodeStats {
		snapshot := nodeStat.Snapshot()
		snapshot.NodeID = nodeID

		for code, count := range snapshot.ErrorCodes {
			aggErrorCodes[code] += count
		}

		detail.NodeMetrics = append(detail.NodeMetrics, NodeMetricDetail{
			NodeID:   nodeID,
			NodeName: nodeNameMap[nodeID],
			Summary: NodeSummaryStats{
				TotalRequests: snapshot.TotalReqs,
				SuccessCount:  snapshot.SuccessReqs,
				FailCount:     snapshot.FailedReqs,
				SuccessRate:   snapshot.SuccessRate,
				AvgLatencyMs:  snapshot.AvgLatency.Seconds() * 1000,
				P50LatencyMs:  snapshot.P50Latency.Seconds() * 1000,
				P90LatencyMs:  snapshot.P90Latency.Seconds() * 1000,
				P95LatencyMs:  snapshot.P95Latency.Seconds() * 1000,
				P99LatencyMs:  snapshot.P99Latency.Seconds() * 1000,
				MinLatencyMs:  snapshot.MinLatency.Seconds() * 1000,
				TTFB:          snapshot.TTFB.Seconds() * 1000,
				AvgQPS:        calculateNodeAvgQPS(snapshot, runRecord.Duration),
				PeakQPS:       collectorData.NodePeakQPS[nodeID],
			},
			TimeSeries: nodeTimeSeriesMap[nodeID],
		})
	}

	for code, count := range aggErrorCodes {
		detail.ErrorSummary = append(detail.ErrorSummary, ErrorItem{
			ErrorType: code,
			Message:   code,
			Count:     count,
		})
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		r.log.Error("failed to marshal report detail", logger.F("error", err))
	}

	reportStatus := model.ReportStatusSuccess
	if runRecord.Status == model.RunStatusFailed {
		reportStatus = model.ReportStatusFailed
	} else if runRecord.FailedReqs > 0 && runRecord.SuccessReqs > 0 {
		reportStatus = model.ReportStatusPartial
	}

	report := &model.Report{
		SceneID:    runRecord.SceneID,
		RunID:      runRecord.ID,
		Status:     reportStatus,
		Summary:    string(summaryBytes),
		Detail:     string(detailJSON),
		StartedAt:  runRecord.StartedAt,
		FinishedAt: runRecord.FinishedAt,
	}

	return r.reports.Create(context.Background(), report)
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
	traceID := r.runID.String()
	execLog := r.log.With(
		logger.F("trace_id", traceID),
		logger.F("scene_id", r.cfg.SceneID.String()),
	)

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
		execLog.Error("failed to create worker pool", logger.F("error", err))
		return fmt.Errorf("runner: create pool: %w", err)
	}
	execLog.Info("worker pool created", logger.F("workers", r.cfg.Workers), logger.F("run_mode", r.cfg.RunMode))

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

		chainID := r.nodeGen.Generate()
		taskCtx = context.WithValue(taskCtx, dag.ChainIDKey, chainID.String())

		var execOpts []dag.ExecutorOption
		if r.tracer != nil {
			hook := &dag.HookAdapter{Tracer: r.tracer}
			execOpts = append(execOpts, dag.WithTraceHook(hook, r.cfg.SceneID, r.runID))
		}
		execOpts = append(execOpts, dag.WithConditionEvaluator(r.evalCondition))

		exec := dag.NewExecutor(dagObj, execOpts...)

		resolvedVars := variable.ResolveAll(scope)
		execLog.Info("resolved scene variables",
			logger.F("chain_id", chainID.String()),
			logger.F("variable_count", len(resolvedVars)),
			logger.F("variables", fmt.Sprintf("%v", resolvedVars)),
		)

		_ = genRegistry
		_ = httpProto
		_ = pluginReg

		_, err := exec.ExecuteWithTrace(taskCtx, resolvedVars)
		if err != nil {
			r.stats.RecordLatency(0, false)
			return err
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

		r.nodeStats[nodeIDStr] = NewNodeStats(10000)

		dagNode, buildErr := r.buildDAGNode(n, r.nodeStats[nodeIDStr])
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
	stats     *Stats
	nodeStats *NodeStats
	log       logger.Logger
	traceID   string
}

func (n *sceneNode) ID() string             { return n.id }
func (n *sceneNode) Timeout() time.Duration { return n.timeout }
func (n *sceneNode) LoopCount() int         { return n.loopCount }
func (n *sceneNode) Mode() dag.ExecMode     { return n.mode }

func (n *sceneNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
	chainID, _ := ctx.Value(dag.ChainIDKey).(string)
	nodeLog := n.log.With(
		logger.F("trace_id", n.traceID),
		logger.F("chain_id", chainID),
		logger.F("node_id", n.id),
		logger.F("node_type", n.nodeType),
	)

	nodeLog.Info("node execution started")

	switch n.nodeType {
	case model.NodeTypeHTTP, model.NodeTypeSetup, model.NodeTypeTeardown:
		return n.executeHTTP(ctx, input, nodeLog)
	case model.NodeTypeDelay:
		return n.executeDelay(ctx, nodeLog)
	case model.NodeTypeCondition:
		return n.executeCondition(input, nodeLog)
	case model.NodeTypeIfElse:
		return n.executeIfElse(input, nodeLog)
	default:
		nodeLog.Warn("unknown node type, skipping")
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": n.nodeType}}, nil
	}
}

func (n *sceneNode) executeHTTP(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
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

	genReg := builtin.DefaultRegistry()
	url = resolveGeneratorRefs(url, genReg, nodeLog)
	nodeLog.Info("resolved generator references in URL", logger.F("url", url))

	if input != nil && input.Variables != nil {
		nodeLog.Info("resolving variables in URL",
			logger.F("original_url", url),
			logger.F("variable_count", len(input.Variables)),
		)
		url = resolveWithVariables(url, input.Variables)
		nodeLog.Info("URL after variable resolution",
			logger.F("resolved_url", url),
		)
	} else {
		nodeLog.Warn("no variables available for URL resolution",
			logger.F("input_nil", input == nil),
			logger.F("variables_nil", input != nil && input.Variables == nil),
		)
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

	for k, v := range req.Headers {
		req.Headers[k] = resolveGeneratorRefs(v, genReg, nodeLog)
	}

	if cfg.Body != "" {
		body := resolveGeneratorRefs(cfg.Body, genReg, nodeLog)
		if input != nil && input.Variables != nil {
			body = resolveWithVariables(body, input.Variables)
		}
		req.Body = []byte(body)
	}

	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(ctx, req)
	if err != nil {
		nodeLog.Error("HTTP request failed",
			logger.F("method", method),
			logger.F("url", url),
			logger.F("error", err),
		)
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		return &dag.Output{Error: err}, nil
	}

	if n.stats != nil {
		if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok {
			n.stats.RecordLatency(httpResp.Latency, httpResp.IsSuccess())
			nodeLog.Info("HTTP request completed",
				logger.F("method", method),
				logger.F("url", url),
				logger.F("status", httpResp.StatusCode),
				logger.F("latency_ms", httpResp.Latency.Milliseconds()),
				logger.F("success", httpResp.IsSuccess()),
			)
		}
	}

	if n.nodeStats != nil {
		if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok {
			n.nodeStats.RecordLatency(httpResp.Latency, httpResp.IsSuccess())
			if !httpResp.IsSuccess() {
				n.nodeStats.RecordError(fmt.Sprintf("HTTP-%d", httpResp.StatusCode))
			}
		}
	}

	return &dag.Output{Response: resp}, nil
}

func (n *sceneNode) executeDelay(ctx context.Context, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		Ms int64 `json:"ms"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		nodeLog.Error("failed to parse delay config", logger.F("error", err))
		return nil, fmt.Errorf("parse delay config: %w", err)
	}

	dur := time.Duration(cfg.Ms) * time.Millisecond
	if dur <= 0 {
		dur = 100 * time.Millisecond
	}

	nodeLog.Info("delay node started",
		logger.F("duration_ms", cfg.Ms),
		logger.F("resolved_duration", dur.String()),
	)

	select {
	case <-time.After(dur):
		if n.stats != nil {
			n.stats.RecordLatency(dur, true)
		}
		if n.nodeStats != nil {
			n.nodeStats.RecordLatency(dur, true)
		}
		nodeLog.Info("delay node completed",
			logger.F("duration_ms", cfg.Ms),
			logger.F("actual_ms", dur.Milliseconds()),
		)
		return &dag.Output{Response: map[string]any{"delay_ms": cfg.Ms}}, nil
	case <-ctx.Done():
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		nodeLog.Warn("delay node cancelled", logger.F("error", ctx.Err()))
		return nil, ctx.Err()
	}
}

func (n *sceneNode) executeCondition(input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		Expr string `json:"expr"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse condition config", logger.F("error", err))
		return nil, fmt.Errorf("parse condition config: %w", err)
	}

	resolvedExpr := cfg.Expr
	if input != nil && input.Variables != nil {
		resolvedExpr = resolveWithVariables(cfg.Expr, input.Variables)
	}

	result := resolvedExpr != "" && resolvedExpr != "false" && resolvedExpr != "0"
	nodeLog.Info("condition node evaluated",
		logger.F("expr", cfg.Expr),
		logger.F("resolved_expr", resolvedExpr),
		logger.F("result", result),
	)
	if n.stats != nil {
		n.stats.RecordLatency(0, true)
	}
	return &dag.Output{Response: map[string]any{"condition": result}}, nil
}

func (n *sceneNode) executeIfElse(input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		Expr string `json:"expr"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse if-else config", logger.F("error", err))
		return nil, fmt.Errorf("parse if-else config: %w", err)
	}

	result := evaluateExpression(cfg.Expr, input)
	nodeLog.Info("if-else node evaluated",
		logger.F("expr", cfg.Expr),
		logger.F("result", result),
		logger.F("branch", map[bool]string{true: "if_true", false: "if_false"}[result]),
	)
	if n.stats != nil {
		n.stats.RecordLatency(0, true)
	}
	return &dag.Output{Response: map[string]any{"if_else_result": result}}, nil
}

func evaluateExpression(expr string, input *dag.Input) bool {
	if expr == "" {
		return true
	}

	if input != nil && input.Variables != nil {
		expr = resolveWithVariables(expr, input.Variables)
	}

	return expr != "" && expr != "false" && expr != "0" && expr != "''" && expr != "\"\""
}

func (r *Runner) evalCondition(ctx context.Context, condition string, output *dag.Output) bool {
	if condition == "" {
		return true
	}

	if condition == "__if_true__" {
		if output != nil {
			if resp, ok := output.Response.(map[string]any); ok {
				if result, exists := resp["if_else_result"]; exists {
					return result == true
				}
			}
		}
		return true
	}

	if condition == "__if_false__" {
		if output != nil {
			if resp, ok := output.Response.(map[string]any); ok {
				if result, exists := resp["if_else_result"]; exists {
					return result != true
				}
			}
		}
		return false
	}

	return condition != "false" && condition != "0"
}

func (r *Runner) buildDAGNode(n *model.Node, nodeStat *NodeStats) (*sceneNode, error) {
	sn := &sceneNode{
		id:        n.ID.String(),
		nodeType:  n.Type,
		config:    n.Config,
		loopCount: n.LoopCount,
		mode:      dag.ExecSync,
		stats:     r.stats,
		nodeStats: nodeStat,
		log:       r.log,
		traceID:   r.runID.String(),
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

func resolveWithVariables(str string, vars map[string]any) string {
	if str == "" || vars == nil {
		return str
	}
	for k, v := range vars {
		str = strings.ReplaceAll(str, "${"+k+"}", fmt.Sprintf("%v", v))
	}
	return str
}

var genSchemaOnce sync.Once
var genNameToSchema map[string]*generator.Schema

func initGeneratorSchemaMap() {
	genSchemaOnce.Do(func() {
		genNameToSchema = make(map[string]*generator.Schema)
		for _, cat := range builtin.Catalog() {
			for _, g := range cat.Generators {
				schema := schemaFromTemplate(g.SchemaTemplate)
				genNameToSchema[g.Name] = schema
			}
		}
	})
}

func schemaFromTemplate(tmpl map[string]any) *generator.Schema {
	if tmpl == nil {
		return &generator.Schema{Type: generator.TypeString}
	}
	s := &generator.Schema{}
	if v, ok := tmpl["type"].(string); ok {
		s.Type = generator.Type(v)
	}
	if v, ok := tmpl["format"].(string); ok {
		s.Format = v
	}
	if v, ok := tmpl["minimum"].(float64); ok {
		s.Minimum = &v
	}
	if v, ok := tmpl["maximum"].(float64); ok {
		s.Maximum = &v
	}
	if v, ok := tmpl["minLength"].(float64); ok {
		n := int(v)
		s.MinLength = &n
	}
	if v, ok := tmpl["maxLength"].(float64); ok {
		n := int(v)
		s.MaxLength = &n
	}
	if v, ok := tmpl["enum"]; ok {
		if arr, ok := v.([]any); ok {
			s.Enum = arr
		}
	}
	if v, ok := tmpl["multipleOf"].(float64); ok {
		s.MultipleOf = &v
	}
	return s
}

var generatorRefRe = regexp.MustCompile(`\$\{generator\.([a-zA-Z0-9_-]+)\}`)

func resolveGeneratorRefs(str string, reg *generator.Registry, log logger.Logger) string {
	if str == "" || reg == nil {
		return str
	}
	initGeneratorSchemaMap()
	return generatorRefRe.ReplaceAllStringFunc(str, func(match string) string {
		name := generatorRefRe.FindStringSubmatch(match)
		if len(name) < 2 {
			return match
		}
		genName := name[1]
		schema, ok := genNameToSchema[genName]
		if !ok {
			return match
		}
		val, err := reg.Generate(schema)
		if err != nil {
			if log != nil {
				log.Warn("generator function failed",
					logger.F("function", genName),
					logger.F("error", err),
				)
			}
			return match
		}
		if log != nil {
			log.Info("generator function called",
				logger.F("function", genName),
				logger.F("output", fmt.Sprintf("%v", val)),
			)
		}
		return fmt.Sprintf("%v", val)
	})
}

// GlobalSnapshot implements StatsProvider interface for global statistics.
func (r *Runner) GlobalSnapshot() *Sample {
	totalReqs := r.stats.TotalReqs.Load()
	successReqs := r.stats.SuccessReqs.Load()
	failedReqs := r.stats.FailedReqs.Load()

	avg, p50, p90, p95, p99 := r.stats.LatencyPercentiles()

	duration := time.Since(r.startedAt).Seconds()
	qps := float64(0)
	if duration > 0 {
		qps = float64(totalReqs) / duration
	}

	return &Sample{
		Timestamp:     time.Now().UTC(),
		WindowSeconds: 1,
		QPS:           qps,
		TotalRequests: totalReqs,
		SuccessCount:  successReqs,
		FailCount:     failedReqs,
		AvgLatencyMs:  avg.Seconds() * 1000,
		P50LatencyMs:  p50.Seconds() * 1000,
		P90LatencyMs:  p90.Seconds() * 1000,
		P95LatencyMs:  p95.Seconds() * 1000,
		P99LatencyMs:  p99.Seconds() * 1000,
		MinLatencyMs:  time.Duration(r.stats.MinLatency.Load()).Seconds() * 1000,
		MaxLatencyMs:  p99.Seconds() * 1000,
	}
}

// NodeSnapshots implements StatsProvider interface for per-node statistics.
func (r *Runner) NodeSnapshots() map[string]*Sample {
	result := make(map[string]*Sample)

	duration := time.Since(r.startedAt).Seconds()

	r.log.Debug("NodeSnapshots called",
		logger.F("node_count", len(r.nodeStats)),
		logger.F("duration_s", duration),
	)

	for nodeID, nodeStat := range r.nodeStats {
		snapshot := nodeStat.Snapshot()
		if snapshot == nil {
			r.log.Warn("nil snapshot for node", logger.F("node_id", nodeID))
			continue
		}

		qps := float64(0)
		if duration > 0 {
			qps = float64(snapshot.TotalReqs) / duration
		}

		result[nodeID] = &Sample{
			Timestamp:     time.Now().UTC(),
			WindowSeconds: 1,
			QPS:           qps,
			TotalRequests: snapshot.TotalReqs,
			SuccessCount:  snapshot.SuccessReqs,
			FailCount:     snapshot.FailedReqs,
			AvgLatencyMs:  snapshot.AvgLatency.Seconds() * 1000,
			P50LatencyMs:  snapshot.P50Latency.Seconds() * 1000,
			P90LatencyMs:  snapshot.P90Latency.Seconds() * 1000,
			P95LatencyMs:  snapshot.P95Latency.Seconds() * 1000,
			P99LatencyMs:  snapshot.P99Latency.Seconds() * 1000,
			MinLatencyMs:  snapshot.MinLatency.Seconds() * 1000,
			MaxLatencyMs:  snapshot.P99Latency.Seconds() * 1000,
		}

		r.log.Debug("collected node snapshot",
			logger.F("node_id", nodeID),
			logger.F("total_reqs", snapshot.TotalReqs),
			logger.F("qps", qps),
			logger.F("p50_ms", snapshot.P50Latency.Seconds()*1000),
		)
	}

	r.log.Info("NodeSnapshots completed",
		logger.F("returned_node_count", len(result)),
	)

	return result
}
