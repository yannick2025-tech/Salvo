package api

import (
	"testing"
)

func TestGenerateEnhancedHTML(t *testing.T) {
	// 测试空JSON
	_, err := GenerateEnhancedHTML("{}")
	if err != nil {
		t.Logf("Empty JSON error: %v", err)
	}

	// 测试有效JSON（字符串ID格式）
	validJSON := `{
		"metadata": {
			"run_id": "123456789",
			"scene_id": "987654321",
			"run_mode": "duration",
			"status": "completed",
			"started_at": "2026-05-15T10:00:00Z",
			"finished_at": "2026-05-15T10:01:00Z",
			"planned_duration": 60,
			"worker_count": 10,
			"duration_sec": 60
		},
		"global_summary": {
			"total_requests": 1000,
			"success_count": 950,
			"fail_count": 50,
			"success_rate": 95.0,
			"avg_latency_ms": 100,
			"p50_latency_ms": 90,
			"p90_latency_ms": 150,
			"p95_latency_ms": 200,
			"p99_latency_ms": 300,
			"min_latency_ms": 50,
			"ttfb": 80,
			"peak_qps": 100,
			"throughput": 16.67
		},
		"global_time_series": [],
		"node_metrics": [],
		"error_summary": []
	}`

	html, err := GenerateEnhancedHTML(validJSON)
	if err != nil {
		t.Fatalf("GenerateEnhancedHTML failed: %v", err)
	}

	if html == "" {
		t.Fatal("Generated HTML is empty")
	}

	t.Logf("Generated HTML length: %d", len(html))

	// 测试数字ID格式（数据库中可能存储的格式）
	numericIDJSON := `{
		"metadata": {
			"run_id": 123456789,
			"scene_id": 987654321,
			"run_mode": "duration",
			"status": "completed",
			"started_at": "2026-05-15T10:00:00Z",
			"finished_at": "2026-05-15T10:01:00Z",
			"planned_duration": 60,
			"worker_count": 10,
			"duration_sec": 60
		},
		"global_summary": {
			"total_requests": 1000,
			"success_count": 950,
			"fail_count": 50,
			"success_rate": 74.6,
			"avg_latency_ms": 100,
			"p50_latency_ms": 90,
			"p90_latency_ms": 150,
			"p95_latency_ms": 200,
			"p99_latency_ms": 300,
			"min_latency_ms": 50,
			"ttfb": 80,
			"peak_qps": 100,
			"throughput": 16.67
		},
		"global_time_series": [],
		"node_metrics": [],
		"error_summary": []
	}`

	html2, err := GenerateEnhancedHTML(numericIDJSON)
	if err != nil {
		t.Fatalf("GenerateEnhancedHTML with numeric IDs failed: %v", err)
	}

	if html2 == "" {
		t.Fatal("Generated HTML with numeric IDs is empty")
	}

	t.Logf("Generated HTML with numeric IDs length: %d", len(html2))
}
