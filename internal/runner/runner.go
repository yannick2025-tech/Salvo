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
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/cascade"
	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
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

// genLogAdapter adapts logger.Logger to the generator.Logger interface so that
// the generator registry can log function-level spans through the runner's logger.
type genLogAdapter struct {
	log logger.Logger
}

func (a *genLogAdapter) Debug(msg string, fields ...any) {
	a.log.Debug(msg, toFields(fields...)...)
}

func (a *genLogAdapter) Info(msg string, fields ...any) {
	a.log.Info(msg, toFields(fields...)...)
}

func (a *genLogAdapter) Error(msg string, fields ...any) {
	a.log.Error(msg, toFields(fields...)...)
}

// toFields converts a variadic any slice to logger.Field slice.
// Pairs of (string, any) are expected, matching the standard log field pattern.
func toFields(pairs ...any) []logger.Field {
	fields := make([]logger.Field, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		fields = append(fields, logger.F(key, pairs[i+1]))
	}
	return fields
}

// RunMode is the execution mode for a test scene.
type RunMode string

const (
	RunModeCount    RunMode = "count"
	RunModeDuration RunMode = "duration"
)

type Config struct {
	SceneID             snowflake.ID
	Workers             int
	RunMode             RunMode
	Count               int64
	Duration            time.Duration
	Timeout             time.Duration
	Variables           map[string]string
	EnableSystemMetrics bool                   // Enable runtime/system metrics collection (default: true)
	ExprRegistry        *expr.FunctionRegistry // Expression engine registry with __so and builtins registered
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

// manualCancelKey is a context key used to propagate the manual-cancel flag
// from the Runner down to sceneNode and the DAG executor. The value stored
// is a *atomic.Bool that is set to true when the user manually stops the run.
// The key type is defined in the dag package (dag.ManualCancelKey) to avoid
// circular imports — both runner and dag reference it.

type Stats struct {
	TotalReqs    atomic.Int64
	SuccessReqs  atomic.Int64
	FailedReqs   atomic.Int64
	CanceledReqs atomic.Int64
	MinLatency   atomic.Int64
	TTFB         atomic.Int64
	latencies    sync.Mutex
	latencyList  []time.Duration
}

// RecordCanceled records a request that was aborted due to manual scene
// cancellation. Canceled requests are NOT counted in TotalReqs (which
// tracks requests that reached a server), keeping the error rate
// (FailedReqs / TotalReqs) consistent with the configured error rate.
func (s *Stats) RecordCanceled() {
	s.CanceledReqs.Add(1)
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
	cfg           Config
	status        atomic.Value
	stats         *Stats
	httpOnlyStats *Stats
	cancel        context.CancelFunc
	ctx           context.Context
	mu            sync.Mutex
	done          chan struct{}
	log           logger.Logger
	runErr        atomic.Value // stores error for Error() method
	// manualCanceled is set to true when the user manually stops the run
	// via Stop(). It is propagated through the context so that sceneNode
	// and the DAG executor can distinguish manual cancellation (→ "canceled"
	// status, excluded from error rate) from request timeouts and other
	// errors (→ "error" status, counted as failures).
	manualCanceled atomic.Bool

	scenes      repo.SceneRepo
	nodes       repo.NodeRepo
	edges       repo.EdgeRepo
	runs        repo.RunRecordRepo
	reports     repo.ReportRepo
	dataSources repo.DataSourceRepo
	tracer      *tracelib.Tracer
	runID       snowflake.ID
	dbRecordID  snowflake.ID // DB-assigned PK after runs.Create(), used for Update/Delete queries
	nodeGen     *snowflake.Node

	startedAt  time.Time
	finishedAt time.Time

	// rowIterators maps data source name to its RowIterator.
	// Each task execution advances the iterator to get the next row.
	rowIterators map[string]*RowIterator

	nodeStats        map[string]*NodeStats
	collector        *TimeSeriesCollector
	tsStore          TimeSeriesStore
	runtimeCollector *RuntimeMetricsCollector
	stopWg           sync.WaitGroup

	// exprReg holds the expression engine registry with __so and builtins registered.
	exprReg *expr.FunctionRegistry
}

func New(cfg Config, scenes repo.SceneRepo, nodes repo.NodeRepo, edges repo.EdgeRepo, runs repo.RunRecordRepo, reports repo.ReportRepo, dataSources repo.DataSourceRepo, tracer *tracelib.Tracer, tsStore TimeSeriesStore, log logger.Logger) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	n, err := snowflake.NewNode(3)
	if err != nil {
		return nil, fmt.Errorf("runner: create snowflake node: %w", err)
	}

	r := &Runner{
		cfg:           cfg,
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		scenes:        scenes,
		nodes:         nodes,
		edges:         edges,
		runs:          runs,
		reports:       reports,
		dataSources:   dataSources,
		tracer:        tracer,
		nodeGen:       n,
		runID:         n.Generate(),
		done:          make(chan struct{}),
		log:           log,
		nodeStats:     make(map[string]*NodeStats),
		exprReg:       cfg.ExprRegistry,
		collector: NewTimeSeriesCollector(TimeSeriesConfig{
			SampleInterval:  1 * time.Second,
			FlushInterval:   10 * time.Second,
			MemoryWindowSec: 300,
			MaxNodes:        100,
		}, n.Generate(), tsStore, nil),
		tsStore:          tsStore,
		runtimeCollector: NewRuntimeMetricsCollector(2*time.Second, cfg.EnableSystemMetrics),
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
	// Propagate the manual-cancel flag through the context tree so that
	// sceneNode and the DAG executor can detect manual stops.
	r.ctx = context.WithValue(r.ctx, dag.ManualCancelKey{}, &r.manualCanceled)
	r.status.Store(StatusRunning)
	r.startedAt = time.Now().UTC()
	r.mu.Unlock()

	traceID := r.runID.String()

	// Create a context with trace context for logging
	runCtx := logger.ContextWithTraceID(r.ctx, traceID)
	runCtx = logger.ContextWithSceneID(runCtx, r.cfg.SceneID.String())

	runLog := r.log.WithContext(runCtx)

	r.collector.SetStatsProvider(r)

	if err := r.collector.Start(r.startedAt); err != nil {
		runLog.Error("failed to start timeseries collector", logger.F("error", err))
	}

	r.runtimeCollector.Start()

	defer func() {
		r.runtimeCollector.Stop()
		if stopErr := r.collector.Stop(); stopErr != nil {
			runLog.Error("failed to stop timeseries collector", logger.F("error", stopErr))
		}
		r.finishedAt = time.Now().UTC()
		if r.Status() != StatusCanceled {
			r.status.Store(StatusDone)
		}
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
		r.createFailedRunRecord(runLog, err, "build_dag")
		return fmt.Errorf("runner: build dag: %w", err)
	}
	runLog.Info("DAG built", logger.F("nodes", len(dagObj.Nodes())), logger.F("edges", len(dagObj.Edges())))

	scope, err := r.buildScope(scene)
	if err != nil {
		r.status.Store(StatusFailed)
		runLog.Error("failed to build scope", logger.F("error", err))
		r.createFailedRunRecord(runLog, err, "build_scope")
		return fmt.Errorf("runner: build scope: %w", err)
	}
	runLog.Info("scope built", logger.F("variable_count", len(scope.Keys())))

	// Load data sources and create row iterators
	if r.dataSources != nil {
		dSources, dsErr := r.dataSources.ListBySceneID(r.ctx, r.cfg.SceneID)
		if dsErr != nil {
			runLog.Warn("failed to load data sources", logger.F("error", dsErr))
		} else if len(dSources) > 0 {
			r.rowIterators = make(map[string]*RowIterator, len(dSources))
			for _, ds := range dSources {
				var rows []map[string]string
				if err := json.Unmarshal([]byte(ds.Rows), &rows); err != nil {
					runLog.Warn("failed to parse data source rows",
						logger.F("name", ds.Name), logger.F("error", err))
					continue
				}
				r.rowIterators[ds.Name] = NewRowIterator(rows)
				runLog.Info("loaded data source",
					logger.F("name", ds.Name),
					logger.F("rows", len(rows)),
				)
			}
		}
	}

	lc := lifecycle.New()
	lc.Register(lifecycle.HookSceneSetup, func(ctx context.Context) error {
		return r.scenes.UpdateStatus(ctx, r.cfg.SceneID, model.SceneStatusRunning)
	})
	lc.Register(lifecycle.HookSceneTeardown, func(ctx context.Context) error {
		return r.scenes.UpdateStatus(ctx, r.cfg.SceneID, model.SceneStatusCompleted)
	})

	if err := lc.Run(r.ctx, lifecycle.HookSceneSetup); err != nil {
		runLog.Error("scene setup lifecycle hook failed", logger.F("error", err))
		r.status.Store(StatusFailed)
		r.createFailedRunRecord(runLog, err, "scene_setup")
		return fmt.Errorf("runner: scene setup: %w", err)
	}
	runLog.Info("lifecycle setup done")

	runRecord := &model.RunRecord{
		Model:       model.Model{ID: r.runID},
		RunID:       r.runID,
		SceneID:     r.cfg.SceneID,
		Status:      model.RunStatusRunning,
		WorkerCount: r.cfg.Workers,
		RunMode:     string(r.cfg.RunMode),
		Duration:    r.cfg.Duration.Seconds(),
		Count:       r.cfg.Count,
		StartedAt:   &r.startedAt,
	}
	if err := r.runs.Create(r.ctx, runRecord); err != nil {
		runLog.Error("failed to create run record", logger.F("error", err))
		r.status.Store(StatusFailed)
		return fmt.Errorf("runner: create run record: %w", err)
	}

	r.mu.Lock()
	r.dbRecordID = runRecord.ID
	r.mu.Unlock()
	runLog.Info("run started", logger.F("run_id", r.runID.String()))

	err = r.execute(dagObj, scope, scene)

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

	if r.Status() == StatusCanceled {
		runLog.Info("skip final save, already saved by Stop()", logger.F("run_id", r.runID))
	} else {
		if err != nil && totalReqs == 0 {
			runRecord.Status = model.RunStatusFailed
			runRecord.ErrorMsg = err.Error()
			r.status.Store(StatusFailed)
		} else if totalReqs > 0 {
			rate := float64(successReqs) / float64(totalReqs)
			if rate < 0.95 {
				runLog.Warn("success rate below 95%% threshold",
					logger.F("rate", fmt.Sprintf("%.1f%%", rate*100)),
					logger.F("total", totalReqs),
					logger.F("success", successReqs),
					logger.F("failed", totalReqs-successReqs))
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
	}

	_ = lc.Run(context.Background(), lifecycle.HookSceneTeardown)

	r.stopWg.Wait()

	runLog.Info("scene run completed",
		logger.F("elapsed_ms", time.Since(r.startedAt).Milliseconds()),
	)

	return err
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		// Mark as manually canceled BEFORE calling cancel() so that
		// in-flight requests can detect this via the context flag and
		// record themselves as "canceled" instead of "failed".
		r.manualCanceled.Store(true)
		r.cancel()
		r.status.Store(StatusCanceled)
		r.stopWg.Add(1)
		go func() {
			r.saveFinalSnapshot()
			r.stopWg.Done()
		}()
	}
}

func (r *Runner) saveFinalSnapshot() {
	r.mu.Lock()
	if r.dbRecordID == 0 || r.runs == nil {
		r.mu.Unlock()
		return
	}
	runID := r.dbRecordID
	sceneID := r.cfg.SceneID
	startedAt := r.startedAt
	r.mu.Unlock()

	time.Sleep(300 * time.Millisecond)

	finishedAt := time.Now().UTC()

	totalReqs := r.stats.TotalReqs.Load()
	successReqs := r.stats.SuccessReqs.Load()
	failedReqs := r.stats.FailedReqs.Load()

	avg, p50, p90, p95, p99 := r.stats.LatencyPercentiles()

	runRecord := &model.RunRecord{
		Model:       model.Model{ID: runID},
		SceneID:     sceneID,
		Status:      model.RunStatusCancelled,
		WorkerCount: r.cfg.Workers,
		RunMode:     string(r.cfg.RunMode),
		Duration:    finishedAt.Sub(startedAt).Seconds(),
		TotalReqs:   totalReqs,
		SuccessReqs: successReqs,
		FailedReqs:  failedReqs,
		AvgLatency:  avg.Seconds(),
		P50Latency:  p50.Seconds(),
		P90Latency:  p90.Seconds(),
		P95Latency:  p95.Seconds(),
		P99Latency:  p99.Seconds(),
		StartedAt:   &startedAt,
		FinishedAt:  &finishedAt,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.runs.Update(ctx, runRecord); err != nil {
		r.log.Error("failed to save final snapshot on stop",
			logger.F("run_id", runID),
			logger.F("error", err))
	} else {
		r.log.Info("final snapshot saved on stop",
			logger.F("run_id", runID),
			logger.F("total_reqs", totalReqs),
			logger.F("success_reqs", successReqs),
			logger.F("failed_reqs", failedReqs),
			logger.F("status", "cancelled"))
	}

	if reportErr := r.createReport(runRecord); reportErr != nil {
		r.log.Error("failed to generate report on stop",
			logger.F("run_id", runID),
			logger.F("error", reportErr))
	} else {
		r.log.Info("report generated on stop",
			logger.F("run_id", runID))
	}
}

func (r *Runner) Status() Status {
	return r.status.Load().(Status)
}

// Error returns the error stored by setError, or nil if no error occurred.
func (r *Runner) Error() error {
	if v := r.runErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// setError stores the error for later retrieval via Error().
// Only the first error is stored.
func (r *Runner) setError(err error) {
	if err == nil {
		return
	}
	r.runErr.CompareAndSwap(nil, err)
}

// createFailedRunRecord creates a run record with Status=failed and the given
// error message. It is called when Runner.Run() fails during early
// initialization steps (buildDAG, buildScope, lifecycle setup) before the
// normal run record is created.
func (r *Runner) createFailedRunRecord(log logger.Logger, runErr error, failedStep string) {
	now := time.Now().UTC()
	record := &model.RunRecord{
		Model:       model.Model{ID: r.runID},
		RunID:       r.runID,
		SceneID:     r.cfg.SceneID,
		Status:      model.RunStatusFailed,
		ErrorMsg:    runErr.Error(),
		WorkerCount: r.cfg.Workers,
		RunMode:     string(r.cfg.RunMode),
		Duration:    r.cfg.Duration.Seconds(),
		Count:       r.cfg.Count,
		StartedAt:   &now,
	}
	if err := r.runs.Create(r.ctx, record); err != nil {
		log.Error("failed to create failed run record",
			logger.F("error", err),
			logger.F("step", failedStep),
		)
	} else {
		r.mu.Lock()
		r.dbRecordID = record.ID
		r.mu.Unlock()
		log.Info("created failed run record for early initialization failure",
			logger.F("step", failedStep),
			logger.F("error_msg", runErr.Error()),
		)
	}
}

func (r *Runner) Stats() *Stats {
	return r.stats
}

func (r *Runner) HttpOnlyStats() *Stats {
	return r.httpOnlyStats
}

func (r *Runner) HttpOnlyGlobalTimeSeries() []Sample {
	if r.collector == nil {
		return nil
	}
	data := r.collector.GetCollectedData()
	return data.HttpOnlyGlobalSamples
}

func (r *Runner) RunID() snowflake.ID {
	return r.runID
}

func (r *Runner) Workers() int {
	return r.cfg.Workers
}

// RuntimeMetricsSnapshots returns the collected runtime metrics snapshots.
// Returns nil if system metrics collection is disabled.
func (r *Runner) RuntimeMetricsSnapshots() []RuntimeMetricsSnapshot {
	if r.runtimeCollector == nil {
		return nil
	}
	return r.runtimeCollector.Snapshots()
}

// RuntimeMetricsSummary returns the aggregated runtime metrics summary.
// Returns an empty summary if system metrics collection is disabled.
func (r *Runner) RuntimeMetricsSummary() SystemMetricsSummary {
	if r.runtimeCollector == nil {
		return SystemMetricsSummary{}
	}
	return r.runtimeCollector.ComputeSummary()
}

// poolStateAdapter adapts a pool.Pool to the RunnerStateProvider
// interface without importing the pool package directly in the
// runtime_metrics.go interface definition.
type poolStateAdapter struct {
	p interface {
		ActiveWorkers() int
		PendingQueueLen() int
	}
}

func (a *poolStateAdapter) ActiveWorkers() int   { return a.p.ActiveWorkers() }
func (a *poolStateAdapter) PendingQueueLen() int { return a.p.PendingQueueLen() }

func (r *Runner) createReport(runRecord *model.RunRecord) error {
	successRate := float64(0)
	if runRecord.TotalReqs > 0 {
		successRate = float64(runRecord.SuccessReqs) / float64(runRecord.TotalReqs) * 100
	}

	summary := map[string]any{
		"scene_id":       runRecord.SceneID,
		"run_id":         runRecord.ID,
		"status":         runRecord.Status,
		"worker_count":   runRecord.WorkerCount,
		"run_mode":       runRecord.RunMode,
		"duration_s":     runRecord.Duration,
		"total_reqs":     runRecord.TotalReqs,
		"success_reqs":   runRecord.SuccessReqs,
		"failed_reqs":    runRecord.FailedReqs,
		"success_rate":   fmt.Sprintf("%.1f%%", successRate),
		"avg_latency_s":  runRecord.AvgLatency,
		"p50_latency_s":  runRecord.P50Latency,
		"p90_latency_s":  runRecord.P90Latency,
		"p95_latency_s":  runRecord.P95Latency,
		"p99_latency_s":  runRecord.P99Latency,
		"min_latency_ms": time.Duration(r.stats.MinLatency.Load()).Seconds() * 1000,
		"p50":            fmt.Sprintf("%.1fms", runRecord.P50Latency*1000),
		"p90":            fmt.Sprintf("%.1fms", runRecord.P90Latency*1000),
		"p95":            fmt.Sprintf("%.1fms", runRecord.P95Latency*1000),
		"p99":            fmt.Sprintf("%.1fms", runRecord.P99Latency*1000),
	}
	summaryBytes, _ := json.Marshal(summary)

	collectorData := r.collector.GetCollectedData()

	r.log.Info("creating report with collected data",
		logger.F("global_samples_count", len(collectorData.GlobalSamples)),
		logger.F("node_samples_count", len(collectorData.NodeSamples)),
	)

	reportCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbRecords, dbErr := r.tsStore.QueryByRunID(reportCtx, r.dbRecordID)
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
		GlobalTimeSeries:         globalTimeSeries,
		HttpOnlyGlobalTimeSeries: collectorData.HttpOnlyGlobalSamples,
		NodeMetrics:              []NodeMetricDetail{},
		ErrorSummary:             collectorData.ErrorItems,
	}

	nodeNameMap := make(map[string]string)
	if nodeList, err := r.nodes.List(reportCtx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000}); err == nil {
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

	// Populate system metrics if available.
	if r.runtimeCollector != nil && r.cfg.EnableSystemMetrics {
		snapshots := r.runtimeCollector.Snapshots()
		summary := r.runtimeCollector.ComputeSummary()
		if len(snapshots) > 0 {
			detail.SystemMetrics = &SystemMetricsData{
				TimeSeries: snapshots,
				Summary:    summary,
			}
		}
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		r.log.Error("failed to marshal report detail", logger.F("error", err))
	}

	reportStatus := model.ReportStatusSuccess
	if runRecord.Status == model.RunStatusFailed {
		reportStatus = model.ReportStatusFailed
	} else if runRecord.TotalReqs > 0 && runRecord.SuccessReqs > 0 {
		successRate := float64(runRecord.SuccessReqs) / float64(runRecord.TotalReqs) * 100
		if successRate >= 95.0 && runRecord.FailedReqs > 0 {
			reportStatus = model.ReportStatusPartial
		} else if successRate < 95.0 {
			reportStatus = model.ReportStatusFailed
		}
	} else if runRecord.FailedReqs > 0 {
		reportStatus = model.ReportStatusFailed
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

func (r *Runner) execute(dagObj *dag.DAG, scope *variable.Scope, scene *model.Scene) error {
	execCtx := logger.ContextWithTraceID(r.ctx, r.runID.String())
	execCtx = logger.ContextWithSceneID(execCtx, r.cfg.SceneID.String())
	execLog := r.log.WithContext(execCtx)

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

	// Connect Pool's wait time stats to the runtime metrics collector.
	r.runtimeCollector.SetWaitTimeStatsProvider(WaitTimeStatsFunc(func() PoolWaitTimeStats {
		ws := p.TaskWaitStats()
		return PoolWaitTimeStats{
			Avg:         ws.Avg,
			P50:         ws.P50,
			P95:         ws.P95,
			P99:         ws.P99,
			Max:         ws.Max,
			SampleCount: ws.SampleCount,
		}
	}))

	// Connect Pool's runner state (active workers, queue length) to the
	// runtime metrics collector.
	r.runtimeCollector.SetRunnerStateProvider(&poolStateAdapter{p: p})

	genRegistry := generator.NewRegistry()
	httpProto := httpprotocol.NewProtocol()
	pluginReg := plugin.NewRegistry()

	task := func(ctx context.Context) error {
		defer func() {
			if p := recover(); p != nil {
				execLog.Error("task panicked",
					logger.F("panic", fmt.Sprintf("%v", p)),
					logger.F("stacktrace", string(debug.Stack())),
				)
				r.stats.RecordLatency(0, false)
			}
		}()

		sceneTimeout := r.cfg.Timeout
			if sceneTimeout <= 0 {
				// 优先使用场景配置的默认超时
				if scene.DefaultTimeout > 0 {
					sceneTimeout = time.Duration(scene.DefaultTimeout) * time.Second
				} else {
					sceneTimeout = 600 * time.Second // 系统默认 10 分钟
				}
			}
			taskCtx, cancel := cascade.NewContext(ctx, r.cfg.SceneID.String(), sceneTimeout)
		defer cancel()

		chainID := r.nodeGen.Generate()
		taskCtx = logger.ContextWithChainID(taskCtx, chainID.String())
		taskCtx = context.WithValue(taskCtx, dag.ChainIDKey, chainID.String())

		// Use context-based logger so chain_id is automatically injected.
		chainLog := r.log.WithContext(taskCtx)
		chainLog.Info("chain execution started")

		var execOpts []dag.ExecutorOption
		if r.tracer != nil {
			hook := &dag.HookAdapter{Tracer: r.tracer}
			execOpts = append(execOpts, dag.WithTraceHook(hook, r.cfg.SceneID, r.runID))
		}
		execOpts = append(execOpts, dag.WithConditionEvaluator(r.evalCondition))
		execOpts = append(execOpts, dag.WithConditionWarnLogger(func(msg string, kv ...any) {
			chainLog.Warn(msg, toFields(kv...)...)
		}))
		execOpts = append(execOpts, dag.WithConditionErrorLogger(func(msg string, kv ...any) {
			chainLog.Error(msg, toFields(kv...)...)
		}))

		exec := dag.NewExecutor(dagObj, execOpts...)

		resolvedVars := variable.ResolveAll(scope)

		// Inject data source row variables: ${ds_name.column} pattern
		for dsName, it := range r.rowIterators {
			row := it.Next()
			for col, val := range row {
				key := dsName + "." + col
				resolvedVars[key] = val
			}
			execLog.Debug("data source row injected",
				logger.F("chain_id", chainID.String()),
				logger.F("data_source", dsName),
				logger.F("row", fmt.Sprintf("%v", row)))
		}

		jsonResolvedVars, _ := json.Marshal(resolvedVars)
		execLog.Info("resolved scene variables",
			logger.F("chain_id", chainID.String()),
			logger.F("variable_count", len(resolvedVars)),
			logger.F("variables", string(jsonResolvedVars)),
		)

		_ = genRegistry
		_ = httpProto
		_ = pluginReg

		_, err := exec.ExecuteWithTrace(taskCtx, resolvedVars)
		if err != nil {
			r.log.Error("DAG execution failed",
				logger.F("error", err),
				logger.F("chain_id", chainID.String()),
			)
			r.stats.RecordLatency(0, false)
			return err
		}

		return nil
	}

	switch r.cfg.RunMode {
	case RunModeCount:
		execLog.Info("worker pool starting", logger.F("mode", "count"), logger.F("count", r.cfg.Count))
		for i := int64(0); i < r.cfg.Count; i++ {
			p.Submit(task)
		}
	case RunModeDuration:
		execLog.Info("worker pool starting", logger.F("mode", "duration"), logger.F("duration", r.cfg.Duration))
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

	err = p.Wait()
	execLog.Info("worker pool stopped",
		logger.F("submitted", p.Submitted()),
		logger.F("completed", p.Completed()),
	)
	return err
}

func (r *Runner) buildDAG(scene *model.Scene) (*dag.DAG, error) {
	dagObj := dag.New()
	buildLog := r.log.With(
		logger.F("trace_id", r.runID.String()),
		logger.F("scene_id", r.cfg.SceneID.String()),
	)

	nodeList, err := r.nodes.List(r.ctx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000})
	if err != nil {
		buildLog.Error("failed to list nodes for DAG", logger.F("error", err))
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// Collect nodes that are referenced as children of any Group node,
	// so they are NOT added as independent DAG nodes — the Group executes them.
	// node_ids in YAML may contain either node names or snowflake IDs (depends
	// on how the scene was imported/saved), so we resolve both to actual Node models.
	groupChildIDs := make(map[string]bool)        // key = node.Name (for skip check)
	groupChildRef := make(map[string]*model.Node) // key = raw value from YAML (name or ID) → resolved Node
	for _, n := range nodeList {
		if n.Type == model.NodeTypeGroup {
			var gcfg struct {
				NodeIDs []string `json:"node_ids"`
			}
			if json.Unmarshal([]byte(n.Config), &gcfg) == nil {
				for _, ref := range gcfg.NodeIDs {
					// Resolve ref → actual Node: try name first, then ID
					var child *model.Node
					for _, cn := range nodeList {
						if cn.Name == ref || cn.ID.String() == ref {
							child = cn
							break
						}
					}
					if child != nil {
						groupChildIDs[child.Name] = true
						groupChildRef[ref] = child
					} else {
						buildLog.Warn("group child reference could not be resolved",
							logger.F("group_name", n.Name),
							logger.F("ref", ref))
					}
				}
			}
		}
	}
	if len(groupChildIDs) > 0 {
		names := make([]string, 0, len(groupChildIDs))
		for name := range groupChildIDs {
			names = append(names, name)
		}
		buildLog.Debug("excluding group child nodes from DAG",
			logger.F("excluded_nodes", names))
	}

	nodeMap := make(map[string]snowflake.ID)
	dagNodeMap := make(map[string]string) // nodeID → nodeName, for diagnostics
	for _, n := range nodeList {
		// Skip Group child nodes — they are executed by their parent Group, not by the DAG directly.
		if groupChildIDs[n.Name] {
			buildLog.Debug("skipping group child node from DAG",
				logger.F("node_id", n.ID.String()),
				logger.F("node_name", n.Name),
				logger.F("node_type", n.Type))
			continue
		}
		nodeIDStr := n.ID.String()
		dagNodeMap[nodeIDStr] = n.Name
		nodeMap[nodeIDStr] = n.ID

		r.nodeStats[nodeIDStr] = NewNodeStats(10000)

		dagNode, buildErr := r.buildDAGNode(n, r.nodeStats[nodeIDStr])
		if buildErr != nil {
			buildLog.Error("failed to build DAG node",
				logger.F("node_id", nodeIDStr),
				logger.F("node_name", n.Name),
				logger.F("node_type", n.Type),
				logger.F("error", buildErr))
			return nil, buildErr
		}
		if addErr := dagObj.AddNode(dagNode); addErr != nil {
			buildLog.Error("failed to add node to DAG",
				logger.F("node_id", nodeIDStr),
				logger.F("error", addErr))
			return nil, fmt.Errorf("add node %s: %w", nodeIDStr, addErr)
		}
	}

	edgeList, err := r.edges.List(r.ctx, repo.Filter{SceneID: r.cfg.SceneID, Limit: 1000})
	if err != nil {
		buildLog.Error("failed to list edges for DAG", logger.F("error", err))
		return nil, fmt.Errorf("list edges: %w", err)
	}

	buildLog.Debug("building DAG edges",
		logger.F("edge_count", len(edgeList)))

	for _, e := range edgeList {
		fromStr := e.FromNode.String()
		toStr := e.ToNode.String()
		edgeType := dag.EdgeNormal
		if e.Condition != "" {
			edgeType = dag.EdgeCondition
		}
		buildLog.Debug("adding edge to DAG",
			logger.F("from", fromStr),
			logger.F("to", toStr),
			logger.F("edge_type", edgeType),
			logger.F("condition", e.Condition))
		if addErr := dagObj.AddEdge(fromStr, toStr, edgeType, e.Condition); addErr != nil {
			buildLog.Error("failed to add edge to DAG",
				logger.F("from", fromStr),
				logger.F("to", toStr),
				logger.F("condition", e.Condition),
				logger.F("error", addErr))
			return nil, fmt.Errorf("add edge %s->%s: %w", fromStr, toStr, addErr)
		}
	}

	// Resolve Group node children: look up child nodes by name from nodeList
	// (not from DAG, since Group children are intentionally excluded from DAG topology)
	// and build sceneNode instances for them.
	for _, n := range nodeList {
		if n.Type != model.NodeTypeGroup {
			continue
		}
		var cfg struct {
			NodeIDs   []string `json:"node_ids"`
			LoopCount int      `json:"loop_count"`
		}
		if err := json.Unmarshal([]byte(n.Config), &cfg); err != nil {
			buildLog.Error("failed to parse group node config",
				logger.F("node_id", n.ID.String()),
				logger.F("node_name", n.Name),
				logger.F("error", err))
			return nil, fmt.Errorf("parse group node %s config: %w", n.ID, err)
		}
		groupNode, ok := dagObj.Node(n.ID.String())
		if !ok {
			continue
		}
		sn, ok := groupNode.(*sceneNode)
		if !ok {
			continue
		}
		for _, childRef := range cfg.NodeIDs {
			// Use pre-resolved reference (supports both name and ID as ref)
			childModel := groupChildRef[childRef]
			if childModel == nil {
				buildLog.Error("group node references non-existent child",
					logger.F("group_node_id", n.ID.String()),
					logger.F("group_name", n.Name),
					logger.F("child_ref", childRef))
				return nil, fmt.Errorf("group node %s references non-existent child %s", n.ID, childRef)
			}
			// Build sceneNode for this child (same as buildDAGNode but not added to DAG)
			childStatKey := childModel.ID.String()
			if _, exists := r.nodeStats[childStatKey]; !exists {
				r.nodeStats[childStatKey] = NewNodeStats(10000)
			}
			childSN := &sceneNode{
				id:            childModel.ID.String(),
				name:          childModel.Name,
				nodeType:      childModel.Type,
				config:        childModel.Config,
				loopCount:     childModel.LoopCount,
				mode:          dag.ExecSync,
				stats:         r.stats,
				httpOnlyStats: r.httpOnlyStats,
				nodeStats:     r.nodeStats[childStatKey],
				log:           r.log,
				traceID:       r.runID.String(),
			}
			if childSN.loopCount <= 0 {
				childSN.loopCount = 1
			}
			// Validate: Group cannot contain another Group
			if childModel.Type == model.NodeTypeGroup {
				buildLog.Error("group node cannot contain another group",
					logger.F("group_node_id", n.ID.String()),
					logger.F("child_ref", childRef))
				return nil, fmt.Errorf("group node %s cannot contain group child %s", n.ID, childRef)
			}
			sn.childNodes = append(sn.childNodes, childSN)
			buildLog.Debug("resolved group child",
				logger.F("group_name", n.Name),
				logger.F("child_ref", childRef),
				logger.F("child_name", childModel.Name),
				logger.F("child_id", childModel.ID.String()),
				logger.F("child_type", childModel.Type))
		}
		if len(sn.childNodes) > 0 {
			buildLog.Info("group node resolved",
				logger.F("group_name", n.Name),
				logger.F("child_count", len(sn.childNodes)),
				logger.F("loop_count", cfg.LoopCount))
		}
	}

	// Log final DAG composition for diagnostics
	buildLog.Info("DAG built successfully",
		logger.F("total_nodes_in_dag", len(dagNodeMap)),
		logger.F("dag_nodes", dagNodeMap))

	return dagObj, nil
}

type sceneNode struct {
	id            string
	name          string
	nodeType      string
	config        string
	timeout       time.Duration
	loopCount     int
	mode          dag.ExecMode
	stats         *Stats
	httpOnlyStats *Stats
	nodeStats     *NodeStats
	log           logger.Logger
	traceID       string
	// executor is set by Runner for generator nodes to write back variables.
	executor *dag.Executor
	// childNodes holds references to child nodes for Group execution.
	// Populated after DAG construction via resolveGroupChildren.
	childNodes []dag.Node
	// subFlowRunner is set by Runner for sub-flow nodes to load and execute
	// a sub-scene dynamically. Nil when not a sub-flow or in tests.
	subFlowRunner func(ctx context.Context, sceneID string, variables map[string]any) (*dag.Output, error)
	// exprReg holds the expression engine registry with __so and builtins registered.
	exprReg *expr.FunctionRegistry
}

func (n *sceneNode) ID() string             { return n.id }
func (n *sceneNode) Timeout() time.Duration { return n.timeout }
func (n *sceneNode) LoopCount() int         { return n.loopCount }
func (n *sceneNode) Mode() dag.ExecMode     { return n.mode }

func (n *sceneNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
	// Use context-based logging so that trace_id, chain_id, node_id etc.
	// are automatically injected by logger.WithContext.
	logCtx := logger.ContextWithNodeID(ctx, n.id)
	nodeLog := n.log.WithContext(logCtx)

	nodeLog.Info("node execution started",
		logger.F("node_name", n.name))

	// Parse retry and extract configs
	retryCfg := n.parseRetryConfig()
	extractCfg := n.parseExtractConfig()

	// Determine max attempts
	maxAttempts := 1
	if retryCfg != nil && retryCfg.MaxAttempts > 1 {
		maxAttempts = retryCfg.MaxAttempts
	}

	var out *dag.Output
	var err error

	// Execute with retry logic
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Apply backoff delay if retrying
		if attempt > 0 && retryCfg != nil {
			backoff := calculateBackoff(retryCfg, attempt-1)
			nodeLog.Info("retry backoff",
				logger.F("attempt", attempt+1),
				logger.F("max_attempts", maxAttempts),
				logger.F("backoff", backoff.String()),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Execute node logic
		out, err = n.executeNodeLogic(ctx, input, nodeLog)

		// Check if we should retry
		if err == nil || !shouldRetry(retryCfg, err) {
			break
		}

		nodeLog.Warn("node execution failed, will retry",
			logger.F("attempt", attempt+1),
			logger.F("max_attempts", maxAttempts),
			logger.F("error", err),
		)
	}

	// Apply extract post-processing if successful
	if err == nil && out != nil && len(extractCfg) > 0 {
		n.applyExtract(out, extractCfg, input, nodeLog)
	}

	if err != nil {
		nodeLog.Error("node execution failed",
			logger.F("error", err),
			logger.F("node_type", n.nodeType),
			logger.F("node_name", n.name),
			logger.F("status", "failed"),
		)
	} else {
		nodeLog.Info("node execution completed",
			logger.F("node_type", n.nodeType),
			logger.F("node_name", n.name),
			logger.F("status", "success"),
		)
	}

	return out, err
}

// executeNodeLogic executes the actual node logic based on node type
func (n *sceneNode) executeNodeLogic(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	switch n.nodeType {
	case model.NodeTypeHTTP, model.NodeTypeSetup, model.NodeTypeTeardown:
		return n.executeHTTP(ctx, input, nodeLog)
	case model.NodeTypeDelay:
		return n.executeDelay(ctx, input, nodeLog)
	case model.NodeTypeCondition:
		return n.executeCondition(input, nodeLog)
	case model.NodeTypeIfElse:
		return n.executeIfElse(input, nodeLog)
	case model.NodeTypeGroup:
		return n.executeGroup(ctx, input, nodeLog)
	case model.NodeTypeWhile:
		return n.executeWhile(ctx, input, nodeLog)
	case model.NodeTypeParallel:
		return n.executeParallel(ctx, input, nodeLog)
	case model.NodeTypeLoop:
		return n.executeLoop(ctx, input, nodeLog)
	case model.NodeTypeSubFlow:
		return n.executeSubFlow(ctx, input, nodeLog)
	case model.NodeTypeTimer:
		return n.executeTimer(ctx, input, nodeLog)
	case model.NodeTypeGenerator:
		return n.executeGenerator(ctx, input, nodeLog)
	default:
		nodeLog.Warn("unknown node type, skipping")
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": n.nodeType}}, nil
	}
}

func (n *sceneNode) executeHTTP(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		Method     string            `json:"method"`
		URL        string            `json:"url"`
		Headers    map[string]string `json:"headers"`
		Body       string            `json:"body"`
		Timeout    any               `json:"timeout"`     // float64 or string (variable ref like ${timeout_ms})
		ExpectBody map[string]any    `json:"expect_body"` // JSON body assertions like {"errorCode": 0}
		Form       *struct {
			Fields map[string]string `json:"fields"`
			Files  map[string]string `json:"files"`
		} `json:"form"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse http config",
			logger.F("error", err),
			logger.F("config_preview", truncateString(n.config, 200)),
		)
		return nil, fmt.Errorf("parse http config: %w", err)
	}

	url := cfg.URL

	genReg := builtin.DefaultRegistry()
	genReg.SetLogger(&genLogAdapter{nodeLog})
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
	// Resolve timeout: support both numeric value and variable reference string
	if cfg.Timeout != nil {
		switch v := cfg.Timeout.(type) {
		case float64:
			if v > 0 {
				timeout = time.Duration(v * float64(time.Second))
			}
		case string:
			// Resolve variable reference, e.g. ${timeout_ms} → "5000"
			resolved := v
			if input != nil && input.Variables != nil {
				resolved = resolveWithVariables(resolved, input.Variables)
			}
			if sec, err := strconv.ParseFloat(resolved, 64); err == nil && sec > 0 {
				timeout = time.Duration(sec * float64(time.Second))
			} else if ms, err := strconv.ParseFloat(resolved, 64); err == nil && ms > 0 {
				// If value looks like milliseconds (e.g. "5000"), treat as seconds
				timeout = time.Duration(ms * float64(time.Millisecond))
			} else {
				nodeLog.Warn("failed to parse timeout value",
					logger.F("raw", v),
					logger.F("resolved", resolved),
					logger.F("err", err),
				)
			}
		case int:
			if v > 0 {
				timeout = time.Duration(float64(v) * float64(time.Second))
			}
		case int64:
			if v > 0 {
				timeout = time.Duration(float64(v) * float64(time.Second))
			}
		default:
			nodeLog.Warn("unsupported timeout type",
				logger.F("type", fmt.Sprintf("%T", cfg.Timeout)),
				logger.F("value", cfg.Timeout),
			)
		}
	}

	nodeLog.Debug("HTTP request fields after variable resolution",
		logger.F("method", method),
		logger.F("timeout_ms", timeout.Milliseconds()),
		logger.F("header_count", len(cfg.Headers)),
	)

	req := &httpprotocol.HTTPRequest{
		Method:  httpprotocol.Method(method),
		URL:     url,
		Headers: cfg.Headers,
		Timeout: timeout,
	}

	for k, v := range req.Headers {
		resolved := resolveGeneratorRefs(v, genReg, nodeLog)
		if input != nil && input.Variables != nil {
			resolved = resolveWithVariables(resolved, input.Variables)
		}
		req.Headers[k] = resolved
	}

	if len(req.Headers) > 0 {
		nodeLog.Debug("resolved headers",
			logger.F("headers", req.Headers),
		)
	}

	if cfg.Body != "" {
		body := resolveGeneratorRefs(cfg.Body, genReg, nodeLog)
		if input != nil && input.Variables != nil {
			body = resolveWithVariables(body, input.Variables)
		}
		req.Body = []byte(body)
		nodeLog.Debug("resolved body",
			logger.F("body_preview", truncateString(body, 300)),
		)
	}

	// multipart/form-data: takes precedence over Body when both present.
	// File paths and field values are variable-resolved the same way as Body.
	if cfg.Form != nil {
		form := &httpprotocol.FormData{
			Fields: make(map[string]string, len(cfg.Form.Fields)),
			Files:  make(map[string]string, len(cfg.Form.Files)),
		}
		for k, v := range cfg.Form.Fields {
			resolved := resolveGeneratorRefs(v, genReg, nodeLog)
			if input != nil && input.Variables != nil {
				resolved = resolveWithVariables(resolved, input.Variables)
			}
			form.Fields[k] = resolved
		}
		for k, v := range cfg.Form.Files {
			resolved := resolveGeneratorRefs(v, genReg, nodeLog)
			if input != nil && input.Variables != nil {
				resolved = resolveWithVariables(resolved, input.Variables)
			}
			form.Files[k] = resolved
		}
		req.Form = form
		nodeLog.Debug("resolved multipart form",
			logger.F("field_count", len(form.Fields)),
			logger.F("file_count", len(form.Files)),
		)
	}

	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(ctx, req)
	if err != nil {
		// Manual scene cancellation: the user clicked Stop. These in-flight
		// requests never reached the server, so they should be recorded as
		// "canceled" (not "failed") to keep the error rate consistent with
		// the configured value. Request-level timeouts (DeadlineExceeded)
		// and other errors are still counted as failures.
		if dag.IsManualCancel(ctx) {
			nodeLog.Warn("HTTP request canceled by manual stop",
				logger.F("method", method),
				logger.F("url", url),
			)
			if n.stats != nil {
				n.stats.RecordCanceled()
			}
			if n.httpOnlyStats != nil {
				n.httpOnlyStats.RecordCanceled()
			}
			if n.nodeStats != nil {
				n.nodeStats.RecordCanceled()
			}
			return &dag.Output{Error: err}, nil
		}

		nodeLog.Error("HTTP request failed",
			logger.F("method", method),
			logger.F("url", url),
			logger.F("error", err),
		)
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		if n.httpOnlyStats != nil {
			n.httpOnlyStats.RecordLatency(0, false)
		}
		return &dag.Output{Error: err}, nil
	}

	if n.stats != nil {
		if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok {
			n.stats.RecordLatency(httpResp.Latency, httpResp.IsSuccess())
			if n.httpOnlyStats != nil {
				n.httpOnlyStats.RecordLatency(httpResp.Latency, httpResp.IsSuccess())
			}
			nodeLog.Info("HTTP request completed",
				logger.F("method", method),
				logger.F("url", url),
				logger.F("status", httpResp.StatusCode),
				logger.F("latency_ms", httpResp.Latency.Milliseconds()),
				logger.F("success", httpResp.IsSuccess()),
			)
			if !httpResp.IsSuccess() && len(httpResp.Body) > 0 {
				nodeLog.Debug("HTTP response body (non-2xx)",
					logger.F("status", httpResp.StatusCode),
					logger.F("body_preview", truncateString(string(httpResp.Body), 500)),
				)
			} else if len(httpResp.Body) > 0 {
				nodeLog.Debug("HTTP response body",
					logger.F("status", httpResp.StatusCode),
					logger.F("body_preview", truncateString(string(httpResp.Body), 500)),
				)
			}
		}
	}

	// Validate expect_body assertions against the JSON response body
	if len(cfg.ExpectBody) > 0 {
		if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok && len(httpResp.Body) > 0 {
			var bodyJSON map[string]any
			if err := json.Unmarshal(httpResp.Body, &bodyJSON); err != nil {
				nodeLog.Error("failed to parse response body for expect_body validation",
					logger.F("error", err),
					logger.F("body_preview", truncateString(string(httpResp.Body), 200)),
				)
			} else {
				for key, expectedVal := range cfg.ExpectBody {
					actualVal, exists := bodyJSON[key]
					if !exists {
						errMsg := fmt.Sprintf("expect_body field %q not found in response", key)
						nodeLog.Error("expect_body validation failed",
							logger.F("field", key),
							logger.F("error", errMsg),
						)
						// Stats already recorded above based on HTTP status; don't double-count.
						return &dag.Output{Error: fmt.Errorf("%s", errMsg)}, nil
					}
					// Compare numeric values (JSON numbers are float64)
					if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedVal) {
						errMsg := fmt.Sprintf("expect_body field %q: expected %v, got %v", key, expectedVal, actualVal)
						nodeLog.Error("expect_body validation failed",
							logger.F("field", key),
							logger.F("expected", expectedVal),
							logger.F("actual", actualVal),
						)
						// Stats already recorded above based on HTTP status; don't double-count.
						return &dag.Output{Error: fmt.Errorf("%s", errMsg)}, nil
					}
				}
				nodeLog.Info("expect_body validation passed",
					logger.F("assertions", cfg.ExpectBody),
				)
			}
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

func (n *sceneNode) executeGenerator(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	startTime := time.Now()

	var cfg struct {
		Expression string `json:"expression"`
		Variable   string `json:"variable"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		nodeLog.Error("failed to parse generator config", logger.F("error", err))
		return nil, fmt.Errorf("parse generator config: %w", err)
	}

	if cfg.Expression == "" || cfg.Variable == "" {
		if n.stats != nil {
			n.stats.RecordLatency(0, false)
		}
		return nil, fmt.Errorf("generator requires expression and variable fields")
	}

	nodeLog.Info("executing generator",
		logger.F("expression", cfg.Expression),
		logger.F("variable", cfg.Variable),
	)

	genReg := builtin.DefaultRegistry()

	// Resolve variables in the expression first
	exprStr := cfg.Expression
	if input != nil && input.Variables != nil {
		exprStr = resolveWithVariables(exprStr, input.Variables)
	}

	// Resolve generator refs (${generator.xxx} patterns)
	result := resolveGeneratorRefs(exprStr, genReg, nodeLog)

	// Resolve expression engine functions (${__so(...)}, ${__random(...)}, etc.)
	// and evaluate pure math expressions (e.g. "1100000001 / 100" after variable
	// resolution). The expression engine's FunctionRegistry handles __so() and
	// other system functions, which resolveGeneratorRefs does not handle.
	if n.exprReg != nil {
		resolved, err := expr.Resolve(result, nil, n.exprReg)
		if err != nil {
			nodeLog.Warn("expression engine resolve failed, using original",
				logger.F("expression", result),
				logger.F("error", err),
			)
			// SO plugin execution failed: record as a failed request so that
			// the request count is consistent with HTTP nodes (the generator
			// counts as one request regardless of success/failure).
			if n.stats != nil {
				n.stats.RecordLatency(0, false)
			}
			return nil, fmt.Errorf("generator expression resolve failed: %w", err)
		}
		result = resolved
	}

	// Generator succeeded: record as a successful request so that the total
	// request count includes generator nodes (consistent with HTTP nodes).
	if n.stats != nil {
		n.stats.RecordLatency(time.Since(startTime), true)
	}

	nodeLog.Info("generator result",
		logger.F("variable", cfg.Variable),
		logger.F("value", result),
	)

	// Write back to variables if executor is available
	if input != nil && input.Executor != nil {
		input.Executor.SetVariable(cfg.Variable, result)
	}

	return &dag.Output{
		Response: result,
		Variables: map[string]any{
			cfg.Variable: result,
		},
	}, nil
}

func (n *sceneNode) executeDelay(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		Ms any `json:"ms"` // float64, int, or string (variable ref like ${delay_ms})
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		if n.nodeStats != nil {
			n.nodeStats.RecordLatency(0, false)
		}
		nodeLog.Error("failed to parse delay config", logger.F("error", err))
		return nil, fmt.Errorf("parse delay config: %w", err)
	}

	var ms int64
	switch v := cfg.Ms.(type) {
	case float64:
		ms = int64(v)
	case int:
		ms = int64(v)
	case int64:
		ms = v
	case string:
		resolved := v
		// Resolve variables first: ${var-a} → value
		if input != nil && input.Variables != nil {
			resolved = resolveWithVariables(resolved, input.Variables)
		}
		// Resolve expressions: ${__random(100, 200)} → value
		if n.exprReg != nil && strings.Contains(resolved, "${__") {
			exprResolved, err := expr.Resolve(resolved, nil, n.exprReg)
			if err != nil {
				nodeLog.Warn("expression resolve failed in delay ms",
					logger.F("expression", resolved),
					logger.F("err", err),
				)
			} else {
				resolved = exprResolved
			}
		}
		// Support math expressions: "600 * 0.25" → "150"
		if mathResult, err := expr.EvalMath(resolved); err == nil {
			resolved = mathResult
		}
		parsed, err := strconv.ParseInt(resolved, 10, 64)
		if err != nil {
			nodeLog.Warn("failed to parse delay ms value",
				logger.F("raw", v),
				logger.F("resolved", resolved),
				logger.F("err", err),
			)
			ms = 100
		} else {
			ms = parsed
		}
	default:
		ms = 100
	}

	dur := time.Duration(ms) * time.Millisecond
	if dur <= 0 {
		dur = 100 * time.Millisecond
	}

	nodeLog.Info("delay node started",
		logger.F("duration_ms", ms),
		logger.F("resolved_duration", dur.String()),
	)

	select {
	case <-time.After(dur):
		if n.nodeStats != nil {
			n.nodeStats.RecordLatency(dur, true)
		}
		nodeLog.Info("delay node completed",
			logger.F("duration_ms", ms),
			logger.F("actual_ms", dur.Milliseconds()),
		)
		return &dag.Output{Response: map[string]any{"delay_ms": ms}}, nil
	case <-ctx.Done():
		if n.nodeStats != nil {
			n.nodeStats.RecordLatency(0, false)
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

	var variables map[string]any
	if input != nil && input.Variables != nil {
		variables = input.Variables
	}
	result := expr.EvaluateConditionExpr(cfg.Expr, variables)
	nodeLog.Info("condition node evaluated",
		logger.F("expr", cfg.Expr),
		logger.F("result", result),
	)
	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(0, true)
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
	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(0, true)
	}
	return &dag.Output{Response: map[string]any{"if_else_result": result}}, nil
}

func (n *sceneNode) executeGroup(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg struct {
		NodeIDs   []string `json:"node_ids"`
		LoopCount int      `json:"loop_count"`
		Async     bool     `json:"async"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse group config", logger.F("error", err))
		return nil, fmt.Errorf("parse group config: %w", err)
	}

	loopCount := cfg.LoopCount
	if loopCount <= 0 {
		loopCount = 1
	}

	if len(n.childNodes) == 0 {
		nodeLog.Warn("group node has no children, skipping")
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": "group", "iterations": 0}}, nil
	}

	nodeLog.Info("executing group",
		logger.F("child_count", len(n.childNodes)),
		logger.F("loop_count", loopCount),
		logger.F("async", cfg.Async),
	)

	var lastOutput *dag.Output
	for i := 0; i < loopCount; i++ {
		if loopCount > 1 {
			nodeLog.Debug("group iteration started",
				logger.F("iteration", i+1),
				logger.F("total", loopCount))
		}
		for _, child := range n.childNodes {
			select {
			case <-ctx.Done():
				nodeLog.Error("group execution cancelled by context",
					logger.F("error", ctx.Err()),
					logger.F("loop", i),
					logger.F("child_id", child.ID()))
				return nil, fmt.Errorf("group execution cancelled: %w", ctx.Err())
			default:
			}

			childLoopCount := child.LoopCount()
			if childLoopCount <= 0 {
				childLoopCount = 1
			}

			for j := 0; j < childLoopCount; j++ {
				output, err := child.Execute(ctx, input)
				if err != nil {
					nodeLog.Error("group child execution failed",
						logger.F("child_id", child.ID()),
						logger.F("loop", i),
						logger.F("error", err),
					)
					return nil, fmt.Errorf("group child %s loop %d: %w", child.ID(), i, err)
				}
				lastOutput = output
			}
		}
	}

	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(0, true)
	}

	return &dag.Output{Response: map[string]any{
		"node_id":    n.id,
		"type":       "group",
		"iterations": loopCount,
		"last_child": lastOutput,
	}}, nil
}

func (n *sceneNode) executeTimer(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	// Accept multiple field names for the duration value: seconds (canonical),
	// delay, duration, interval — so YAML configs using any of them work.
	var cfg struct {
		Mode     string  `json:"mode"`
		Seconds  float64 `json:"seconds"`
		Delay    float64 `json:"delay"`
		Duration float64 `json:"duration"`
		Interval float64 `json:"interval"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse timer config", logger.F("error", err))
		return nil, fmt.Errorf("parse timer config: %w", err)
	}

	// Pick the first non-zero value from the aliased fields.
	seconds := cfg.Seconds
	if seconds <= 0 && cfg.Delay > 0 {
		seconds = cfg.Delay
	}
	if seconds <= 0 && cfg.Duration > 0 {
		seconds = cfg.Duration
	}
	if seconds <= 0 && cfg.Interval > 0 {
		seconds = cfg.Interval
	}

	if seconds <= 0 {
		nodeLog.Warn("timer duration is zero or missing, using default 1s",
			logger.F("raw_seconds", cfg.Seconds),
			logger.F("raw_delay", cfg.Delay),
			logger.F("raw_duration", cfg.Duration),
			logger.F("raw_interval", cfg.Interval),
			logger.F("mode", cfg.Mode))
		seconds = 1.0
	}

	duration := time.Duration(seconds * float64(time.Second))
	startTime := time.Now()

	switch cfg.Mode {
	case "delay":
		nodeLog.Info("timer delay started", logger.F("seconds", seconds))
		select {
		case <-time.After(duration):
			nodeLog.Info("timer delay completed")
		case <-ctx.Done():
			nodeLog.Info("timer delay cancelled")
			elapsed := time.Since(startTime)
			if n.nodeStats != nil {
				n.nodeStats.RecordLatency(elapsed, false)
			}
			return nil, fmt.Errorf("timer cancelled: %w", ctx.Err())
		}
	case "interval":
		nodeLog.Info("timer interval started", logger.F("seconds", seconds))
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		tickCount := 0
		for {
			select {
			case <-ticker.C:
				tickCount++
				nodeLog.Info("timer tick", logger.F("tick", tickCount))
			case <-ctx.Done():
				nodeLog.Info("timer interval stopped", logger.F("ticks", tickCount))
				elapsed := time.Since(startTime)
				if n.nodeStats != nil {
					n.nodeStats.RecordLatency(elapsed, true)
				}
				return &dag.Output{Response: map[string]any{
					"node_id": n.id,
					"type":    "timer",
					"mode":    "interval",
					"ticks":   tickCount,
				}}, nil
			}
		}
	default:
		nodeLog.Warn("invalid timer mode",
			logger.F("mode", cfg.Mode),
			logger.F("valid_modes", []string{"delay", "interval"}))
		if n.nodeStats != nil {
			n.nodeStats.RecordLatency(0, false)
		}
		return nil, fmt.Errorf("invalid timer mode %q, must be \"delay\" or \"interval\"", cfg.Mode)
	}

	// delay mode completed successfully
	elapsed := time.Since(startTime)
	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(elapsed, true)
	}

	return &dag.Output{Response: map[string]any{
		"node_id": n.id,
		"type":    "timer",
		"mode":    cfg.Mode,
	}}, nil
}

func evaluateExpression(exprStr string, input *dag.Input) bool {
	if exprStr == "" {
		return true
	}

	var variables map[string]any
	if input != nil && input.Variables != nil {
		variables = input.Variables
	}

	return expr.EvaluateConditionExpr(exprStr, variables)
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

	// For generic DAG edge conditions, build variables from the upstream
	// node's output response (if it's a map) and evaluate the expression.
	var variables map[string]any
	if output != nil {
		if resp, ok := output.Response.(map[string]any); ok {
			variables = resp
		}
	}

	return expr.EvaluateConditionExpr(condition, variables)
}

func (r *Runner) buildDAGNode(n *model.Node, nodeStat *NodeStats) (*sceneNode, error) {
	sn := &sceneNode{
		id:            n.ID.String(),
		name:          n.Name,
		nodeType:      n.Type,
		config:        n.Config,
		loopCount:     n.LoopCount,
		mode:          dag.ExecSync,
		stats:         r.stats,
		httpOnlyStats: r.httpOnlyStats,
		nodeStats:     nodeStat,
		log:           r.log,
		traceID:       r.runID.String(),
		exprReg:       r.exprReg,
	}

	if n.LoopCount <= 0 {
		sn.loopCount = 1
	}

	// Timer nodes are always async — they should never block the DAG main chain.
	if n.Type == model.NodeTypeTimer {
		sn.mode = dag.ExecAsync
	}

	return sn, nil
}

func (r *Runner) buildScope(scene *model.Scene) (*variable.Scope, error) {
	scopeLog := r.log.With(
		logger.F("trace_id", r.runID.String()),
		logger.F("scene_id", r.cfg.SceneID.String()),
	)

	globalScope := variable.NewScope(variable.WithLevel(variable.ScopeGlobal))

	if scene.Variables != "" {
		var vars map[string]string
		if err := json.Unmarshal([]byte(scene.Variables), &vars); err != nil {
			scopeLog.Error("failed to parse scene variables JSON",
				logger.F("error", err),
				logger.F("variables_preview", truncateString(scene.Variables, 200)))
			return nil, fmt.Errorf("parse scene variables: %w", err)
		}
		for k, v := range vars {
			globalScope.Set(k, v)
		}
		jsonVars, _ := json.Marshal(vars)
		scopeLog.Debug("scene variables loaded",
			logger.F("variable_count", len(vars)),
			logger.F("variables", string(jsonVars)))
	}

	for k, v := range r.cfg.Variables {
		globalScope.Set(k, v)
	}

	// Resolve nested variable references in all variable values.
	if err := resolveNestedVariables(globalScope); err != nil {
		scopeLog.Error("failed to resolve nested variable references", logger.F("error", err))
		return nil, fmt.Errorf("resolve nested variables: %w", err)
	}

	sceneScope := variable.NewScope(
		variable.WithLevel(variable.ScopeScene),
		variable.WithParent(globalScope),
	)

	return sceneScope, nil
}

// resolveNestedVariables iterates over all variables in a scope and resolves
// any ${var} references in their values using the scope itself.
func resolveNestedVariables(scope *variable.Scope) error {
	keys := scope.Keys()
	for _, k := range keys {
		val, _ := scope.Get(k)
		strVal := fmt.Sprintf("%v", val)
		if !strings.Contains(strVal, "${") {
			continue
		}
		resolved, err := variable.ResolveString(scope, strVal)
		if err != nil {
			return fmt.Errorf("variable %q: %w", k, err)
		}
		scope.Set(k, resolved)
	}
	return nil
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
	canceledReqs := r.stats.CanceledReqs.Load()

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
		CanceledCount: canceledReqs,
		AvgLatencyMs:  avg.Seconds() * 1000,
		P50LatencyMs:  p50.Seconds() * 1000,
		P90LatencyMs:  p90.Seconds() * 1000,
		P95LatencyMs:  p95.Seconds() * 1000,
		P99LatencyMs:  p99.Seconds() * 1000,
		MinLatencyMs:  time.Duration(r.stats.MinLatency.Load()).Seconds() * 1000,
		MaxLatencyMs:  p99.Seconds() * 1000,
	}
}

func (r *Runner) HttpOnlySnapshot() *Sample {
	totalReqs := r.httpOnlyStats.TotalReqs.Load()
	successReqs := r.httpOnlyStats.SuccessReqs.Load()
	failedReqs := r.httpOnlyStats.FailedReqs.Load()
	canceledReqs := r.httpOnlyStats.CanceledReqs.Load()

	avg, p50, p90, p95, p99 := r.httpOnlyStats.LatencyPercentiles()

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
		CanceledCount: canceledReqs,
		AvgLatencyMs:  avg.Seconds() * 1000,
		P50LatencyMs:  p50.Seconds() * 1000,
		P90LatencyMs:  p90.Seconds() * 1000,
		P95LatencyMs:  p95.Seconds() * 1000,
		P99LatencyMs:  p99.Seconds() * 1000,
		MinLatencyMs:  time.Duration(r.httpOnlyStats.MinLatency.Load()).Seconds() * 1000,
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

// truncateString returns a truncated version of s, limited to maxLen characters.
// If s exceeds maxLen, it appends "..." to indicate truncation.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// isSensitiveHeader returns true for headers that carry secrets/tokens.
func isSensitiveHeader(key string) bool {
	lower := strings.ToLower(key)
	switch lower {
	case "authorization", "cookie", "x-api-key", "x-auth-token",
		"x-csrf-token", "proxy-authorization", "set-cookie":
		return true
	default:
		return strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key")
	}
}

// maskValue masks a sensitive value, keeping first 4 and last 4 chars.
func maskValue(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "****" + v[len(v)-4:]
}
