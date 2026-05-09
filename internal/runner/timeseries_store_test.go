package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

func TestTimeSeriesStore_BatchInsert(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(12345)
	now := time.Now()

	records := []TimeSeriesRecord{
		{
			RunID:          runID,
			NodeID:         "",
			SampleTime:     now,
			WindowDuration: 1,
			QPS:            100.5,
			TotalRequests:  100,
			SuccessCount:   95,
			FailCount:      5,
			AvgLatencyMs:   50.0,
			P50LatencyMs:   40.0,
			P95LatencyMs:   80.0,
			P99LatencyMs:   90.0,
			MinLatencyMs:   10.0,
			MaxLatencyMs:   100.0,
		},
		{
			RunID:          runID,
			NodeID:         "node-1",
			SampleTime:     now.Add(1 * time.Second),
			WindowDuration: 1,
			QPS:            80.0,
			TotalRequests:  80,
			SuccessCount:   78,
			FailCount:      2,
			AvgLatencyMs:   45.0,
			P50LatencyMs:   35.0,
			P95LatencyMs:   70.0,
			P99LatencyMs:   85.0,
			MinLatencyMs:   15.0,
			MaxLatencyMs:   95.0,
		},
	}

	err := store.BatchInsert(context.Background(), records)
	if err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}
}

func TestTimeSeriesStore_QueryByRunID(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(99999)
	now := time.Now()

	insertRecords := []TimeSeriesRecord{}
	for i := 0; i < 10; i++ {
		insertRecords = append(insertRecords, TimeSeriesRecord{
			RunID:         runID,
			NodeID:        "",
			SampleTime:    now.Add(time.Duration(i) * time.Second),
			QPS:           float64(100 + i*10),
			TotalRequests: int64(100 + i*10),
			SuccessCount:  int64(95 + i*9),
			FailCount:     int64(5 + i),
			AvgLatencyMs:  float64(50 + i),
		})
	}

	err := store.BatchInsert(context.Background(), insertRecords)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	results, err := store.QueryByRunID(context.Background(), runID)
	if err != nil {
		t.Fatalf("QueryByRunID failed: %v", err)
	}

	if len(results) != 10 {
		t.Errorf("expected 10 records, got %d", len(results))
	}

	for _, r := range results {
		if r.RunID != runID {
			t.Errorf("expected RunID %d, got %d", runID, r.RunID)
		}
	}
}

func TestTimeSeriesStore_QueryByNodeID(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(88888)
	nodeID := "test-node"
	now := time.Now()

	globalRecords := []TimeSeriesRecord{
		{RunID: runID, NodeID: "", SampleTime: now, QPS: 100},
		{RunID: runID, NodeID: "", SampleTime: now.Add(time.Second), QPS: 110},
	}

	nodeRecords := []TimeSeriesRecord{
		{RunID: runID, NodeID: nodeID, SampleTime: now, QPS: 50},
		{RunID: runID, NodeID: nodeID, SampleTime: now.Add(time.Second), QPS: 55},
		{RunID: runID, NodeID: nodeID, SampleTime: now.Add(2 * time.Second), QPS: 60},
	}

	err := store.BatchInsert(context.Background(), append(globalRecords, nodeRecords...))
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	results, err := store.QueryByNodeID(context.Background(), runID, nodeID)
	if err != nil {
		t.Fatalf("QueryByNodeID failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 node records, got %d", len(results))
	}

	for _, r := range results {
		if r.NodeID != nodeID {
			t.Errorf("expected NodeID %s, got %s", nodeID, r.NodeID)
		}
	}
}

func TestTimeSeriesStore_DeleteByRunID(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(77777)
	now := time.Now()

	records := []TimeSeriesRecord{
		{RunID: runID, NodeID: "", SampleTime: now, QPS: 100},
		{RunID: runID, NodeID: "node-1", SampleTime: now, QPS: 50},
	}

	err := store.BatchInsert(context.Background(), records)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	err = store.DeleteByRunID(context.Background(), runID)
	if err != nil {
		t.Fatalf("DeleteByRunID failed: %v", err)
	}

	results, err := store.QueryByRunID(context.Background(), runID)
	if err != nil {
		t.Fatalf("QueryByRunID after delete failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(results))
	}
}

func TestTimeSeriesStore_EmptyQuery(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	results, err := store.QueryByRunID(context.Background(), snowflake.ID(11111))
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty result for non-existent run, got %d", len(results))
	}
}

func TestTimeSeriesStore_ConcurrentOperations(t *testing.T) {
	store := newTestTimeSeriesStore(t)
	defer store.Close()

	runID := snowflake.ID(66666)
	ctx := context.Background()

	var wg sync.WaitGroup
	const writers = 5
	const readers = 5
	const opsPerGoroutine = 20

	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				records := []TimeSeriesRecord{
					{
						RunID:         runID,
						NodeID:        "",
						SampleTime:    time.Now(),
						QPS:           float64(i),
						TotalRequests: int64(i),
					},
				}
				_ = store.BatchInsert(ctx, records)
			}
		}(w)
	}

	wg.Add(readers)
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				_, _ = store.QueryByRunID(ctx, runID)
			}
		}()
	}

	wg.Wait()
}
