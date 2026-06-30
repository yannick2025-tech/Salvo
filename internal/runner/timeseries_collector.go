package runner

import (
	"context"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

// Sample represents a time-series data point collected from stats.
type Sample struct {
	Timestamp     time.Time `json:"t"`
	WindowSeconds int       `json:"dur"`

	QPS            float64 `json:"qps"`
	TotalRequests  int64   `json:"total"`
	SuccessCount   int64   `json:"success"`
	FailCount      int64   `json:"fail"`
	CanceledCount  int64   `json:"canceled"`

	AvgLatencyMs float64 `json:"avg_ms"`
	P50LatencyMs float64 `json:"p50_ms"`
	P90LatencyMs float64 `json:"p90_ms"`
	P95LatencyMs float64 `json:"p95_ms"`
	P99LatencyMs float64 `json:"p99_ms"`
	MinLatencyMs float64 `json:"min_ms"`
	MaxLatencyMs float64 `json:"max_ms"`
}

// TimeSeriesConfig holds configuration for the time series collector.
type TimeSeriesConfig struct {
	SampleInterval  time.Duration
	FlushInterval   time.Duration
	MemoryWindowSec int
	MaxNodes        int
}

// StatsProvider provides statistics snapshots for collection.
type StatsProvider interface {
	GlobalSnapshot() *Sample
	HttpOnlySnapshot() *Sample
	NodeSnapshots() map[string]*Sample
}

// CollectedData contains all collected time-series data.
type CollectedData struct {
	GlobalSamples        []Sample            `json:"global_samples"`
	HttpOnlyGlobalSamples []Sample            `json:"http_only_global_samples"`
	NodeSamples          map[string][]Sample `json:"node_samples"`
	GlobalPeakQPS        float64             `json:"global_peak_qps"`
	NodePeakQPS          map[string]float64  `json:"node_peak_qps"`
	ErrorItems           []ErrorItem         `json:"error_items,omitempty"`
}

// ErrorItem represents an aggregated error occurrence.
type ErrorItem struct {
	NodeID    string    `json:"node_id,omitempty"`
	ErrorType string    `json:"error_type"`
	Message   string    `json:"message"`
	Count     int64     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// TimeSeriesCollector collects and persists time-series metrics.
type TimeSeriesCollector struct {
	cfg  TimeSeriesConfig
	runID snowflake.ID
	store TimeSeriesStore
	log   *log.Logger

	mu                      sync.RWMutex
	globalSamples           []Sample
	httpOnlyGlobalSamples   []Sample
	nodeSamples             map[string][]Sample
	pendingFlush            []TimeSeriesRecord
	prevGlobalReqs          int64
	prevNodeReqs            map[string]int64

	statsProvider StatsProvider
	startTime     time.Time
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewTimeSeriesCollector creates a new time series collector with the given configuration.
func NewTimeSeriesCollector(cfg TimeSeriesConfig, runID snowflake.ID, store TimeSeriesStore, logger *log.Logger) *TimeSeriesCollector {
	if cfg.SampleInterval == 0 {
		cfg.SampleInterval = 1 * time.Second
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.MemoryWindowSec == 0 {
		cfg.MemoryWindowSec = 300
	}
	if cfg.MaxNodes == 0 {
		cfg.MaxNodes = 100
	}

	if logger == nil {
		logger = log.New(log.Writer(), "[timeseries] ", log.LstdFlags)
	}

	return &TimeSeriesCollector{
		cfg:                    cfg,
		runID:                  runID,
		store:                  store,
		log:                    logger,
		globalSamples:          make([]Sample, 0),
		httpOnlyGlobalSamples:  make([]Sample, 0),
		nodeSamples:            make(map[string][]Sample),
		pendingFlush:           make([]TimeSeriesRecord, 0),
		prevNodeReqs:           make(map[string]int64),
		stopCh:                 make(chan struct{}),
	}
}

// SetStatsProvider sets the stats provider for collecting snapshots.
func (c *TimeSeriesCollector) SetStatsProvider(provider StatsProvider) {
	c.statsProvider = provider
}

// Start begins the sampling and flushing goroutines.
func (c *TimeSeriesCollector) Start(startTime time.Time) error {
	c.startTime = startTime
	c.stopCh = make(chan struct{})

	c.wg.Add(1)
	go c.sampleLoop()

	c.wg.Add(1)
	go c.flushLoop()

	return nil
}

// Stop gracefully stops the collector and performs final flush.
func (c *TimeSeriesCollector) Stop() error {
	close(c.stopCh)
	c.wg.Wait()
	c.flush()
	return nil
}

// GetCollectedData returns a copy of all collected data.
func (c *TimeSeriesCollector) GetCollectedData() *CollectedData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := &CollectedData{
		GlobalSamples:        make([]Sample, len(c.globalSamples)),
		HttpOnlyGlobalSamples: make([]Sample, len(c.httpOnlyGlobalSamples)),
		NodeSamples:          make(map[string][]Sample),
	}

	copy(data.GlobalSamples, c.globalSamples)
	copy(data.HttpOnlyGlobalSamples, c.httpOnlyGlobalSamples)

	for nodeID, samples := range c.nodeSamples {
		data.NodeSamples[nodeID] = make([]Sample, len(samples))
		copy(data.NodeSamples[nodeID], samples)
	}

	data.GlobalPeakQPS = calculatePeakQPS(c.globalSamples)
	data.NodePeakQPS = calculatePeakQPSPerNode(c.nodeSamples)

	return data
}

func (c *TimeSeriesCollector) sampleLoop() {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[timeseries] sampleLoop panicked: %v\n%s", r, debug.Stack())
		}
	}()

	ticker := time.NewTicker(c.cfg.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case now := <-ticker.C:
			c.takeSnapshot(now)
		}
	}
}

func (c *TimeSeriesCollector) flushLoop() {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[timeseries] flushLoop panicked: %v\n%s", r, debug.Stack())
		}
	}()

	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.flush()
		}
	}
}

func (c *TimeSeriesCollector) takeSnapshot(now time.Time) {
	if c.statsProvider == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	globalSnap := c.statsProvider.GlobalSnapshot()
	if globalSnap != nil && globalSnap.Timestamp.IsZero() {
		globalSnap.Timestamp = now
	}

	nodeSnaps := c.statsProvider.NodeSnapshots()

	if globalSnap != nil {
		c.globalSamples = append(c.globalSamples, *globalSnap)
		c.trimSamplesBefore(now.Add(-time.Duration(c.cfg.MemoryWindowSec) * time.Second))

		record := TimeSeriesRecord{
			RunID:          c.runID,
			SampleTime:     globalSnap.Timestamp,
			WindowDuration: globalSnap.WindowSeconds,
			QPS:           globalSnap.QPS,
			TotalRequests: globalSnap.TotalRequests,
			SuccessCount:  globalSnap.SuccessCount,
			FailCount:     globalSnap.FailCount,
			AvgLatencyMs:  globalSnap.AvgLatencyMs,
			P50LatencyMs:  globalSnap.P50LatencyMs,
			P90LatencyMs:  globalSnap.P90LatencyMs,
			P95LatencyMs:  globalSnap.P95LatencyMs,
			P99LatencyMs:  globalSnap.P99LatencyMs,
			MinLatencyMs:  globalSnap.MinLatencyMs,
			MaxLatencyMs:  globalSnap.MaxLatencyMs,
		}
		c.pendingFlush = append(c.pendingFlush, record)
	}

	httpOnlySnap := c.statsProvider.HttpOnlySnapshot()
	if httpOnlySnap != nil && httpOnlySnap.Timestamp.IsZero() {
		httpOnlySnap.Timestamp = now
	}
	if httpOnlySnap != nil {
		c.httpOnlyGlobalSamples = append(c.httpOnlyGlobalSamples, *httpOnlySnap)
	}

	for nodeID, snap := range nodeSnaps {
		if snap == nil || snap.Timestamp.IsZero() {
			snap.Timestamp = now
		}

		if _, exists := c.nodeSamples[nodeID]; !exists {
			c.nodeSamples[nodeID] = make([]Sample, 0)
		}
		c.nodeSamples[nodeID] = append(c.nodeSamples[nodeID], *snap)

		nodeRecord := TimeSeriesRecord{
			RunID:          c.runID,
			NodeID:         nodeID,
			SampleTime:     snap.Timestamp,
			WindowDuration: snap.WindowSeconds,
			QPS:           snap.QPS,
			TotalRequests: snap.TotalRequests,
			SuccessCount:  snap.SuccessCount,
			FailCount:     snap.FailCount,
			AvgLatencyMs:  snap.AvgLatencyMs,
			P50LatencyMs:  snap.P50LatencyMs,
			P90LatencyMs:  snap.P90LatencyMs,
			P95LatencyMs:  snap.P95LatencyMs,
			P99LatencyMs:  snap.P99LatencyMs,
			MinLatencyMs:  snap.MinLatencyMs,
			MaxLatencyMs:  snap.MaxLatencyMs,
		}
		c.pendingFlush = append(c.pendingFlush, nodeRecord)
	}
}

func (c *TimeSeriesCollector) trimSamplesBefore(cutoff time.Time) {
	c.globalSamples = trimSamples(c.globalSamples, cutoff)
	c.httpOnlyGlobalSamples = trimSamples(c.httpOnlyGlobalSamples, cutoff)
	for nodeID := range c.nodeSamples {
		c.nodeSamples[nodeID] = trimSamples(c.nodeSamples[nodeID], cutoff)
	}
}

func (c *TimeSeriesCollector) flush() {
	c.mu.Lock()
	if len(c.pendingFlush) == 0 {
		c.mu.Unlock()
		return
	}

	batch := make([]TimeSeriesRecord, len(c.pendingFlush))
	copy(batch, c.pendingFlush)
	c.pendingFlush = c.pendingFlush[:0]
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.store.BatchInsert(ctx, batch); err != nil {
		c.log.Printf("batch insert failed: count=%d error=%v", len(batch), err)
	} else {
		c.log.Printf("flushed %d samples", len(batch))
	}
}

func trimSamples(samples []Sample, cutoff time.Time) []Sample {
	for i, s := range samples {
		if s.Timestamp.After(cutoff) {
			return samples[i:]
		}
	}
	return samples[len(samples):]
}

func calculatePeakQPS(samples []Sample) float64 {
	max := 0.0
	for _, s := range samples {
		if s.QPS > max {
			max = s.QPS
		}
	}
	return max
}

func calculatePeakQPSPerNode(nodeSamples map[string][]Sample) map[string]float64 {
	result := make(map[string]float64)
	for nodeID, samples := range nodeSamples {
		result[nodeID] = calculatePeakQPS(samples)
	}
	return result
}
