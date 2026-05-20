package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/runner"
)

var enhancedReportTemplate = template.Must(template.New("enhanced-report").Funcs(template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return "--"
		}
		return t.Local().Format("2006-01-02 15:04:05")
	},
	"formatTimeShort": func(t string) string {
		if len(t) >= 19 {
			return t[11:19]
		}
		return t
	},
	"formatSuccessRate": func(rate interface{}) string {
		var r float64
		switch v := rate.(type) {
		case float64:
			r = v
		case int64:
			r = float64(v)
		case float32:
			r = float64(v)
		default:
			return "0.00%"
		}
		return fmt.Sprintf("%.2f%%", r)
	},
	"formatLatencyMs": func(val interface{}) string {
		var ms float64
		switch v := val.(type) {
		case float64:
			ms = v
		case float32:
			ms = float64(v)
		default:
			return "--"
		}
		if ms <= 0 {
			return "--"
		}
		if ms < 1000 {
			return fmt.Sprintf("%.3fms", ms)
		}
		return fmt.Sprintf("%.3fs", ms/1000)
	},
	"formatQPS": func(qps interface{}) string {
		var q float64
		switch v := qps.(type) {
		case float64:
			q = v
		case float32:
			q = float64(v)
		default:
			return "--"
		}
		if q <= 0 {
			return "--"
		}
		return fmt.Sprintf("%.1f", q)
	},
	"formatNumber": func(n interface{}) string {
		switch v := n.(type) {
		case float64:
			if v == 0 {
				return "0"
			}
			return fmt.Sprintf("%.0f", v)
		case int64:
			return fmt.Sprintf("%d", v)
		case int:
			return fmt.Sprintf("%d", v)
		default:
			return "0"
		}
	},
	"add": func(a, b int) int {
		return a + b
	},
	"now": func() time.Time {
		return time.Now()
	},
}).Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>测试报告 - Salvo</title>
    <script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-secondary: #f9fafb;
            --bg-card: #ffffff;
            --bg-tertiary: #f3f4f6;
            --bg-hover: rgba(8,145,178,0.04);
            --text-primary: #111827;
            --text-secondary: #374151;
            --text-tertiary: #6b7280;
            --border-primary: #e5e7eb;
            --border-secondary: #e5e7eb;
            --accent-primary: #0891b2;
            --accent-success: #16a34a;
            --accent-danger: #dc2626;
            --accent-warning: #ca8a04;
            --accent-info: #2563eb;
            --radius-sm: 6px;
            --radius-md: 8px;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            line-height: 1.5;
            padding: 20px;
        }

        .container {
            max-width: 1280px;
            margin: 0 auto;
            display: flex;
            flex-direction: column;
            gap: 20px;
        }

        /* Page Header */
        .page-header {
            display: flex;
            align-items: center;
            gap: 12px;
            padding-bottom: 16px;
            border-bottom: 1px solid var(--border-secondary);
        }
        .page-header h2 {
            font-size: 18px;
            font-weight: 600;
            flex: 1;
        }

        /* Metrics Row */
        .metrics-row {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: clamp(8px, 1.5vw, 16px);
            margin-bottom: 16px;
        }
        .metric-card {
            background: var(--bg-card);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            padding: 16px 18px;
            text-align: center;
        }
        .metric-label {
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.06em;
            color: var(--text-tertiary);
            margin-bottom: 6px;
        }
        .metric-value {
            font-size: 24px;
            font-weight: 700;
            line-height: 1.2;
        }
        .metric-sub {
            font-size: 11px;
            color: var(--text-tertiary);
            margin-top: 4px;
        }

        /* Chart Cards */
        .charts-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 16px;
            margin-bottom: 16px;
        }
        .chart-card {
            background: var(--bg-card);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            overflow: hidden;
        }
        .chart-card.wide {
            grid-column: span 2;
        }
        .chart-header {
            padding: 14px 16px 10px;
            display: flex;
            align-items: baseline;
            justify-content: space-between;
            flex-wrap: wrap;
        }
        .chart-header h3 {
            font-size: 13px;
            font-weight: 600;
            color: var(--text-secondary);
        }
        .chart-tip {
            font-size: 11px;
            color: var(--text-tertiary);
        }
        .chart-body {
            height: 260px;
        }

        /* Info Section */
        .info-section {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 16px;
        }
        .info-card {
            background: var(--bg-card);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            padding: 16px;
        }
        .info-card h3 {
            font-size: 13px;
            font-weight: 600;
            color: var(--text-secondary);
            margin-bottom: 12px;
        }
        .info-table {
            width: 100%;
            border-collapse: collapse;
        }
        .info-table td {
            padding: 7px 0;
            font-size: 13px;
            border-bottom: 1px solid var(--border-secondary);
        }
        .info-table tr:last-child td {
            border-bottom: none;
        }
        .info-label {
            color: var(--text-secondary);
            min-width: 110px;
        }
        .mono-sm {
            font-family: monospace;
            font-size: 12px;
        }

        .mode-tag {
            display: inline-block;
            padding: 2px 10px;
            border-radius: var(--radius-sm);
            font-size: 12px;
            font-weight: 600;
            background: rgba(8,145,178,0.1);
            color: var(--accent-primary);
        }
        .actual-val {
            color: var(--accent-success);
            font-weight: 500;
        }

        .status-badge {
            font-size: 11px;
            padding: 3px 10px;
            border-radius: var(--radius-sm);
            font-weight: 600;
            letter-spacing: 0.04em;
        }
        .status-badge.success { background: rgba(22,163,74,0.15); color: #16a34a; }
        .status-badge.partial { background: rgba(217,119,6,0.15); color: #d97706; }
        .status-badge.failed { background: rgba(220,38,38,0.15); color: #dc2626; }
        .status-badge.running { background: rgba(8,145,178,0.15); color: #0891b2; }
        .status-badge.pending { background: rgba(139,148,158,0.15); color: #656d76; }
        .status-badge.canceled { background: rgba(139,148,158,0.12); color: #8b949e; }

        /* Performance List */
        .perf-list {
            display: flex;
            flex-direction: column;
            gap: 6px;
        }
        .perf-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 7px 12px;
            background: var(--bg-tertiary);
            border-radius: var(--radius-sm);
        }
        .perf-k {
            font-size: 12px;
            font-weight: 600;
            color: var(--text-tertiary);
            min-width: 48px;
        }
        .perf-v {
            font-size: 13px;
            font-weight: 600;
            color: var(--text-primary);
            font-variant-numeric: tabular-nums;
        }
        .perf-v.success { color: var(--accent-success); }
        .perf-v.danger { color: var(--accent-danger); }
        .perf-v.highlight-blue { color: var(--accent-info); }
        .perf-divider {
            height: 1px;
            background: var(--border-secondary);
            margin: 4px 0;
        }

        /* Charts Section */
        .charts-section {
            background: var(--bg-card);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            padding: 16px;
            margin-bottom: 16px;
        }
        .charts-toolbar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }
        .charts-toolbar h3 {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
        }

        /* Node Ranking Table */
        .ranking-table-wrap {
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
        }
        .ranking-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 12px;
        }
        .ranking-table thead th {
            padding: 10px 12px;
            text-align: left;
            font-weight: 600;
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.04em;
            color: var(--text-tertiary);
            border-bottom: 2px solid var(--border-secondary);
            white-space: nowrap;
            background: var(--bg-tertiary);
        }
        .ranking-table tbody td {
            padding: 9px 12px;
            border-bottom: 1px solid var(--border-secondary);
            color: var(--text-secondary);
            font-variant-numeric: tabular-nums;
            white-space: nowrap;
        }
        .ranking-table tbody tr:hover td {
            background: var(--bg-hover);
        }
        .rank-cell {
            font-weight: 700;
            color: var(--text-tertiary);
            width: 32px;
        }
        .node-name-cell {
            font-weight: 500;
            color: var(--text-primary);
            max-width: 180px;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .success-text { color: var(--accent-success); font-weight: 600; }
        .danger-text { color: var(--accent-danger); font-weight: 600; }

        /* Nodes Section */
        .nodes-section {
            background: var(--bg-card);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            padding: 16px;
            margin-bottom: 16px;
        }
        .nodes-section h3 {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
            margin-bottom: 12px;
        }
        .node-badges {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
        }
        .node-badge {
            padding: 2px 10px;
            border-radius: var(--radius-sm);
            font-size: 11px;
            font-weight: 600;
            background: var(--bg-tertiary);
            color: var(--text-secondary);
        }
        .node-chart-body {
            height: 280px;
        }

        /* Chart Type Toggle */
        .chart-type-toggle {
            display: flex;
            gap: 6px;
            justify-content: center;
            margin-top: 8px;
            padding-bottom: 4px;
        }
        .type-btn {
            padding: 3px 12px;
            border: 1px solid var(--border-primary);
            border-radius: var(--radius-sm);
            background: transparent;
            color: var(--text-secondary);
            font-size: 11px;
            cursor: pointer;
            transition: all 0.15s ease;
        }
        .type-btn.active {
            background: var(--accent-primary);
            color: #fff;
            border-color: var(--accent-primary);
        }
        .type-btn:hover:not(.active) {
            background: var(--bg-hover);
            border-color: var(--accent-primary);
        }

        @media (max-width: 1100px) {
            .metrics-row { grid-template-columns: repeat(4, 1fr); gap: 10px; }
        }
        @media (max-width: 900px) {
            .info-section { grid-template-columns: 1fr; }
            .charts-row { grid-template-columns: 1fr; }
            .chart-card.wide { grid-column: span 1; }
            .metrics-row { grid-template-columns: repeat(2, 1fr); gap: 10px; }
        }
        @media (max-width: 520px) {
            .metrics-row { grid-template-columns: 1fr 1fr; gap: 8px; }
        }
    </style>
</head>
<body>
<div class="container">
    <div class="page-header">
        <h2>测试报告</h2>
    </div>

    {{if .Metrics}}
    <!-- Metrics Row -->
    <div class="metrics-row">
        <div class="metric-card">
            <div class="metric-label">成功率</div>
            <div class="metric-value" style="color: {{if gt .Metrics.SuccessRate 90.0}}#16a34a{{else if gt .Metrics.SuccessRate 70.0}}#d97706{{else}}#dc2626{{end}}">{{formatSuccessRate .Metrics.SuccessRate}}</div>
            <div class="metric-sub">{{formatNumber .Metrics.SuccessCount}} / {{formatNumber .Metrics.TotalRequests}} 次请求</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">总请求数</div>
            <div class="metric-value" style="color: var(--accent-primary)">{{formatNumber .Metrics.TotalRequests}}</div>
            <div class="metric-sub">{{if .Metrics.DurationS}}{{printf "%.1f" .Metrics.DurationS}}s{{else}}--{{end}} 持续时间</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">峰值QPS</div>
            <div class="metric-value" style="color: var(--accent-info)">{{formatQPS .Metrics.PeakQPS}}</div>
            <div class="metric-sub">{{formatQPS .Metrics.AvgQPS}} 平均QPS</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">吞吐量</div>
            <div class="metric-value" style="color: var(--accent-success)">{{formatQPS .Metrics.Throughput}}/s</div>
            <div class="metric-sub">{{.Metrics.WorkerCount}} 工作线程</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">平均延迟</div>
            <div class="metric-value" style="color: var(--accent-info)">{{formatLatencyMs .Metrics.AvgLatencyMs}}</div>
            <div class="metric-sub">P50 {{formatLatencyMs .Metrics.P50LatencyMs}}</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">P90延迟</div>
            <div class="metric-value" style="color: var(--accent-warning)">{{formatLatencyMs .Metrics.P90LatencyMs}}</div>
            <div class="metric-sub">P95 {{formatLatencyMs .Metrics.P95LatencyMs}}</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">首字节时间</div>
            <div class="metric-value" style="color: var(--accent-primary)">{{formatLatencyMs .Metrics.TTFBMs}}</div>
            <div class="metric-sub">最小值 {{formatLatencyMs .Metrics.MinLatencyMs}}</div>
        </div>

        <div class="metric-card">
            <div class="metric-label">P99延迟</div>
            <div class="metric-value" style="color: var(--accent-danger)">{{formatLatencyMs .Metrics.P99LatencyMs}}</div>
            <div class="metric-sub">{{printf "%.2f%%" .Metrics.ErrorRate}} 错误率</div>
        </div>
    </div>

    <!-- Charts Row: Request Distribution + Error Rate Trend -->
    <div class="charts-row">
        <div class="chart-card">
            <div class="chart-header">
                <h3>请求分布</h3>
                <div class="chart-tip">成功/失败分布</div>
            </div>
            <div class="chart-body" id="overviewChart"></div>
        </div>

        <div class="chart-card">
            <div class="chart-header">
                <h3>错误率趋势</h3>
                <div class="chart-tip">每区间失败/总数</div>
            </div>
            <div class="chart-body" id="errorRateChart"></div>
            <div class="chart-type-toggle">
                <button class="type-btn active" onclick="switchChartType('errorRate', 'smooth')">平滑</button>
                <button class="type-btn" onclick="switchChartType('errorRate', 'step')">阶梯</button>
            </div>
        </div>
    </div>

    {{if gt (len .ErrorBreakdown) 0}}
    <!-- Error Breakdown -->
    <div class="charts-row">
        <div class="chart-card wide">
            <div class="chart-header">
                <h3>错误明细</h3>
                <div class="chart-tip">按HTTP状态码分组</div>
            </div>
            <div class="chart-body" id="errorBreakdownChart"></div>
        </div>
    </div>
    {{end}}

    <!-- Latency Percentiles Bar Chart -->
    <div class="charts-row">
        <div class="chart-card wide">
            <div class="chart-header">
                <h3>延迟百分位</h3>
                <div class="chart-tip">均值/P50/P90/P95/P99 (ms)</div>
            </div>
            <div class="chart-body" id="latencyChart"></div>
        </div>
    </div>

    <!-- Run Info & Performance Summary -->
    <div class="info-section">
        <div class="info-card">
            <h3>运行时配置</h3>
            <table class="info-table">
                <tr><td class="info-label">运行模式</td><td><span class="mode-tag">{{.Metadata.RunMode}}</span></td></tr>
                {{if eq .Metadata.RunMode "duration"}}
                <tr><td class="info-label">计划持续时间</td><td>{{.Metadata.PlannedDuration}}s</td></tr>
                {{end}}
                {{if eq .Metadata.RunMode "count"}}
                <tr><td class="info-label">计划请求数</td><td>{{.Metadata.PlannedCount}} 次</td></tr>
                {{end}}
                <tr><td class="info-label">实际{{if eq .Metadata.RunMode "duration"}}持续时间{{else}}请求数{{end}}</td><td class="actual-val">{{if eq .Metadata.RunMode "duration"}}{{printf "%.2f" .Metrics.DurationS}}s{{else}}{{formatNumber .Metrics.TotalRequests}} 次{{end}}</td></tr>
                <tr><td class="info-label">并发数</td><td>{{.Metrics.WorkerCount}}</td></tr>
                <tr><td class="info-label">状态</td><td><span class="status-badge {{.Metadata.Status}}">{{.Metadata.StatusLabel}}</span></td></tr>
                <tr><td class="info-label">时间范围</td><td class="mono-sm">{{formatTime .Metadata.StartedAt}} ~ {{formatTime .Metadata.FinishedAt}}</td></tr>
            </table>
        </div>

        <div class="info-card">
            <h3>性能概览</h3>
            <div class="perf-list">
                <div class="perf-row"><span class="perf-k">均值</span><span class="perf-v">{{formatLatencyMs .Metrics.AvgLatencyMs}}</span></div>
                <div class="perf-row"><span class="perf-k">P50</span><span class="perf-v">{{formatLatencyMs .Metrics.P50LatencyMs}}</span></div>
                <div class="perf-row"><span class="perf-k">P90</span><span class="perf-v">{{formatLatencyMs .Metrics.P90LatencyMs}}</span></div>
                <div class="perf-row"><span class="perf-k">P95</span><span class="perf-v">{{formatLatencyMs .Metrics.P95LatencyMs}}</span></div>
                <div class="perf-row"><span class="perf-k">P99</span><span class="perf-v">{{formatLatencyMs .Metrics.P99LatencyMs}}</span></div>
                <div class="perf-row"><span class="perf-k">最小值</span><span class="perf-v">{{formatLatencyMs .Metrics.MinLatencyMs}}</span></div>
                <div class="perf-divider"></div>
                <div class="perf-row"><span class="perf-k">TTFB</span><span class="perf-v">{{formatLatencyMs .Metrics.TTFBMs}}</span></div>
                <div class="perf-row"><span class="perf-k">峰值QPS</span><span class="perf-v highlight-blue">{{formatQPS .Metrics.PeakQPS}}</span></div>
                <div class="perf-row"><span class="perf-k">吞吐量</span><span class="perf-v success">{{formatQPS .Metrics.Throughput}}/s</span></div>
                <div class="perf-divider"></div>
                <div class="perf-row"><span class="perf-k">成功数</span><span class="perf-v success">{{formatNumber .Metrics.SuccessCount}}</span></div>
                <div class="perf-row"><span class="perf-k">失败数</span><span class="perf-v danger">{{formatNumber .Metrics.FailCount}}</span></div>
            </div>
        </div>
    </div>

    <!-- Trend Charts Section -->
    <section class="charts-section">
        <div class="charts-toolbar">
            <h3>趋势分析</h3>
        </div>

        <div class="charts-row">
            <div class="chart-card wide">
                <div class="chart-header">
                    <h3>QPS趋势</h3>
                </div>
                <div class="chart-body" id="qpsChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('qpsTrend', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('qpsTrend', 'step')">阶梯</button>
                </div>
            </div>
        </div>

        <div class="charts-row">
            <div class="chart-card wide">
                <div class="chart-header">
                    <h3>延迟趋势</h3>
                    <div class="chart-tip">P50 / P90 / P95 / P99</div>
                </div>
                <div class="chart-body" id="latencyTrendChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('latTrend', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('latTrend', 'step')">阶梯</button>
                </div>
            </div>
        </div>
    </section>

    {{if gt (len .NodeMetrics) 0}}
    <!-- Node Ranking Table -->
    <section class="nodes-section">
        <h3>节点排名</h3>
        <div class="ranking-table-wrap">
            <table class="ranking-table">
                <thead>
                    <tr>
                        <th>排名</th>
                        <th>节点</th>
                        <th>总请求</th>
                        <th>成功</th>
                        <th>失败</th>
                        <th>成功率</th>
                        <th>P50</th>
                        <th>P90</th>
                        <th>P95</th>
                        <th>平均QPS</th>
                        <th>峰值QPS</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $idx, $node := .NodeMetrics}}
                    <tr>
                        <td class="rank-cell">{{add $idx 1}}</td>
                        <td class="node-name-cell">{{$node.Name}}</td>
                        <td>{{formatNumber $node.Summary.TotalRequests}}</td>
                        <td class="success-text">{{formatNumber $node.Summary.SuccessCount}}</td>
                        <td class="danger-text">{{formatNumber $node.Summary.FailCount}}</td>
                        <td>{{printf "%.2f%%" $node.Summary.SuccessRate}}</td>
                        <td>{{formatLatencyMs $node.Summary.P50LatencyMs}}</td>
                        <td>{{formatLatencyMs $node.Summary.P90LatencyMs}}</td>
                        <td>{{formatLatencyMs $node.Summary.P95LatencyMs}}</td>
                        <td>{{$node.Summary.AvgQPS | formatQPS}}</td>
                        <td>{{$node.Summary.PeakQPS | formatQPS}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </section>

    <!-- Node Details with charts -->
    <section class="nodes-section">
        <h3>节点详情</h3>
        {{range $idx, $node := .NodeMetrics}}
        <div class="chart-card" style="margin-bottom: 16px;">
            <div class="chart-header">
                <h3>{{$node.Name}}</h3>
                <div class="node-badges">
                    <span class="node-badge">QPS {{$node.Summary.AvgQPS | formatQPS}}</span>
                    <span class="node-badge">P50 {{formatLatencyMs $node.Summary.P50LatencyMs}}</span>
                    <span class="node-badge">P90 {{formatLatencyMs $node.Summary.P90LatencyMs}}</span>
                    <span class="node-badge">P95 {{formatLatencyMs $node.Summary.P95LatencyMs}}</span>
                    <span class="node-badge">TTFB {{formatLatencyMs $node.Summary.TTFBMs}}</span>
                </div>
            </div>
            <div class="chart-body node-chart-body" id="nodeChart{{$idx}}"></div>
            <div class="chart-type-toggle">
                <button class="type-btn active" onclick="switchChartType('node-{{$idx}}', 'smooth')">平滑</button>
                <button class="type-btn" onclick="switchChartType('node-{{$idx}}', 'step')">阶梯</button>
            </div>
        </div>
        {{end}}
    </section>
    {{end}}

    {{end}}

    <footer style="text-align: center; padding: 2rem; color: var(--text-tertiary);">
        <p>Generated by Salvo at {{now.Format "2006-01-02 15:04:05"}}</p>
    </footer>
</div>

<script>
// Data from server
const reportData = {{.JSONData}};

// Theme colors
const tc = {
    bg: '#ffffff',
    textColor: '#111827',
    lineColor: '#e5e7eb',
    colors: ['#0891b2', '#0891b2', '#d97706', '#dc2626'],
    dangerColor: '#dc2626'
};

let errorRateType = 'smooth';
let qpsType = 'smooth';
let latTrendType = 'smooth';
let nodeType = 'smooth';

const chartTypes = { errorRate: 'smooth', qpsTrend: 'smooth', latTrend: 'smooth' };

function initNodeChartTypes() {
    if (reportData.node_metrics) {
        reportData.node_metrics.forEach(function(_, idx) {
            chartTypes['node-' + idx] = 'smooth';
        });
    }
}

function switchChartType(chartId, type) {
    chartTypes[chartId] = type;
    if (chartId === 'errorRate') { updateToggleButtons('errorRateChart'); renderErrorRateChart(); }
    else if (chartId === 'qpsTrend') { updateToggleButtons('qpsChart'); renderQPSTrend(); }
    else if (chartId === 'latTrend') { updateToggleButtons('latencyTrendChart'); renderLatencyTrend(); }
    else if (chartId.startsWith('node-')) { updateNodeToggleButtons(chartId); renderNodeCharts(); }
}

function updateNodeToggleButtons(chartId) {
    const idx = parseInt(chartId.slice(5));
    const el = document.getElementById('nodeChart' + idx);
    if (!el) return;
    const card = el.closest('.chart-card');
    if (!card) return;
    const btns = card.querySelectorAll('.type-btn');
    btns.forEach(function(btn) {
        btn.classList.toggle('active', btn.textContent.trim().toLowerCase() === chartTypes[chartId]);
    });
}

function initCharts() {
    initNodeChartTypes();
    renderOverviewChart();
    renderErrorRateChart();
    renderLatencyChart();
    renderQPSTrend();
    renderLatencyTrend();
    renderNodeCharts();
    {{if gt (len .ErrorBreakdown) 0}}
    renderErrorBreakdownChart();
    {{end}}
}

function renderOverviewChart() {
    const m = reportData.metrics || {};
    const chart = echarts.init(document.getElementById('overviewChart'));
    
    const total = Number(m.total_reqs || 0);
    const success = Number(m.success_reqs || 0);
    const failed = Number(m.failed_reqs || 0);
    
    chart.setOption({
        backgroundColor: tc.bg,
        color: [tc.colors[1], tc.colors[3]],
        tooltip: {
            trigger: 'item',
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.97)',
            borderColor: 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.12);',
            formatter: function(p) {
                const pct = ((p.value / Math.max(total, 1)) * 100).toFixed(1);
                return '<div style="font-weight:600;margin-bottom:4px">' + p.name + '</div>' +
                       '<div style="font-size:13px"><strong>' + p.value.toLocaleString() + '</strong> requests (' + pct + '%)</div>';
            }
        },
        graphic: [
            {
                type: 'text',
                left: '30%',
                top: '44%',
                style: {
                    text: total.toLocaleString(),
                    fontSize: 22,
                    fontWeight: 700,
                    fill: tc.textColor,
                    textAlign: 'center'
                }
            },
            {
                type: 'text',
                left: '30%',
                top: '58%',
                style: {
                    text: '总请求',
                    fontSize: 11,
                    fill: '#8c959f',
                    textAlign: 'center'
                }
            }
        ],
        legend: {
            orient: 'vertical', right: 20, top: 'center',
            itemWidth: 12, itemHeight: 12, itemGap: 14,
            icon: 'roundRect',
            formatter: function(name) {
                const val = name === 'Success' ? success : failed;
                const pct = (val / Math.max(total, 1) * 100).toFixed(1);
                return '{name|' + name + '}   {val|' + pct + '%}';
            },
            textStyle: {
                rich: {
                    name: { color: '#24292f', fontWeight: 500, fontSize: 13 },
                    val: { color: '#8c959f', fontSize: 12, fontWeight: 400 }
                }
            }
        },
        series: [{
            type: 'pie',
            radius: ['50%', '72%'],
            center: ['36%', '50%'],
            avoidLabelOverlap: false,
            startAngle: 90,
            padAngle: 2,
            itemStyle: {
                borderRadius: 8,
                borderColor: tc.bg,
                borderWidth: 3
            },
            label: { show: false },
            emphasis: {
                scale: true,
                scaleSize: 8,
                itemStyle: { shadowBlur: 20, shadowColor: 'rgba(0,0,0,0.15)' },
                label: { show: true, fontSize: 14, fontWeight: 'bold', color: tc.textColor }
            },
            data: [
                { value: success, name: 'Success', itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
                    { offset: 0, color: '#16a34a' }, { offset: 1, color: '#15803d' }
                ])}},
                { value: failed, name: 'Failed', itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
                    { offset: 0, color: '#dc2626' }, { offset: 1, color: '#b91c1c' }
                ])}}
            ]
        }]
    });
}

function renderErrorRateChart() {
    const m = reportData.metrics || {};
    const timestamps = m.timestamps || [];
    const totals = m.ts_total || [];
    const fails = m.ts_fail || [];
    
    const errRates = [];
    for (let i = 0; i < totals.length; i++) {
        errRates.push(totals[i] > 0 ? (fails[i] / totals[i]) * 100 : 0);
    }
    
    if (!timestamps.length || !errRates.length) return;
    
    const timeLabels = timestamps.map(ts => ts.substring(11, 19));
    const maxErrRate = Math.max(...errRates, 0.01);
    const globalErrRate = ((Number(m.failed_reqs || 0) / Math.max(Number(m.total_reqs || 1), 1)) * 100);
    
    const chart = echarts.init(document.getElementById('errorRateChart'));
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.96)',
            borderColor: 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 8,
            padding: [10, 14],
            textStyle: { fontSize: 11, color: '#475569' },
            formatter: function(params) {
                const idx = params[0].dataIndex;
                return timeLabels[idx] + '<br/>Error Rate: <strong>' + params[0].value.toFixed(2) + '%</strong><br/>' +
                       'Failed: ' + (fails[idx] || 0) + ' / Total: ' + (totals[idx] || 0);
            }
        },
        grid: { left: 50, right: 16, top: 20, bottom: 44 },
        dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(220,38,38,0.10)', handleStyle: { color: tc.dangerColor }, textStyle: { color: tc.textColor, fontSize: 9 }, showDetail: false }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 9, interval: Math.floor(timeLabels.length / 8) } },
        yAxis: { type: 'value', name: '%', min: 0, max: Math.max(maxErrRate * 1.5, globalErrRate * 1.5, 1), axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(2) + '%' } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
        markLine: {
            silent: true,
            data: [{ yAxis: globalErrRate, label: { formatter: 'Total: ' + globalErrRate.toFixed(2) + '%', color: tc.dangerColor, fontSize: 9 }, lineStyle: { color: tc.dangerColor, type: 'dashed', width: 1 } }],
            symbol: 'none'
        },
        series: [{
            name: 'Error Rate',
            type: 'line',
            smooth: chartTypes.errorRate === 'smooth',
            step: chartTypes.errorRate === 'step' ? 'middle' : false,
            data: errRates,
            lineStyle: { width: 2, color: tc.dangerColor },
            areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: 'rgba(220,38,38,0.18)' }, { offset: 1, color: 'rgba(220,38,38,0.01)' }
            ])},
            symbol: 'none'
        }]
    });
}

function renderLatencyChart() {
    const m = reportData.metrics || {};
    const chart = echarts.init(document.getElementById('latencyChart'));
    
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.97)',
            borderColor: 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.10);',
            formatter: function(params) {
                const labels = { Avg: '平均延迟', P50: '中位数 (50%)', P90: 'P90 延迟', P95: 'P95 延迟', P99: 'P99 尾部延迟' };
                return '<div style="font-weight:600;margin-bottom:4px">' + (labels[params[0].name] || params[0].name) + '</div>' +
                       '<div style="font-size:13px"><strong>' + params[0].value.toFixed(1) + '</strong> ms</div>';
            }
        },
        grid: { left: 44, right: 20, top: 24, bottom: 32 },
        xAxis: {
            type: 'category',
            data: ['Avg', 'P50', 'P90', 'P95', 'P99'],
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: '#656d76', fontSize: 12, fontWeight: 600, margin: 12 }
        },
        yAxis: {
            type: 'value',
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: '#8c959f', fontSize: 10, formatter: function(v) { return v.toFixed(0) } },
            splitLine: { lineStyle: { color: 'rgba(208,215,222,0.6)', type: 'dashed' } }
        },
        series: [{
            type: 'bar',
            barWidth: '40%',
            data: [
                { value: parseFloat(m.avg_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: '#0891b2' }, { offset: 1, color: '#0e7490' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p50_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: '#0891b2' }, { offset: 1, color: '#0e7490' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p90_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: '#d97706' }, { offset: 1, color: '#b45309' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p95_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: '#d97706' }, { offset: 1, color: '#b45309' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p99_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: '#8b5cf6' }, { offset: 1, color: '#7c3aed' }
                ]), borderRadius: [6, 6, 2, 2] }}
            ],
            label: {
                show: true, position: 'top',
                formatter: function(p) { return p.value.toFixed(1); },
                color: '#656d76', fontSize: 11, fontWeight: 600, offset: [0, -4]
            }
        }]
    });
}

function renderQPSTrend() {
    const m = reportData.metrics || {};
    const timestamps = m.timestamps || [];
    const qpsData = m.ts_qps || [];
    
    if (!timestamps.length || !qpsData.length) return;
    
    const timeLabels = timestamps.map(ts => ts.substring(11, 19));
    const isSmooth = chartTypes.qpsTrend === 'smooth';
    
    const chart = echarts.init(document.getElementById('qpsChart'));
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.96)',
            borderColor: 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 12,
            padding: [12, 16],
            textStyle: { fontSize: 11, color: '#475569' },
            extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
            formatter: function(params) {
                const idx = params[0].dataIndex;
                return '<div style="font-size:11.5px;color:#1e293b;margin-bottom:6px;font-weight:600">' + timeLabels[idx] + '</div>' +
                       'QPS: <strong>' + params[0].value.toFixed(1) + '</strong>';
            }
        },
        grid: { left: 50, right: 20, top: 20, bottom: 50 },
        dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(8,145,178, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(1) } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
        series: [{
            name: 'QPS',
            type: 'line',
            smooth: isSmooth,
            step: isSmooth ? false : 'middle',
            data: qpsData,
            lineStyle: { width: 2, color: tc.colors[0] },
            itemStyle: { color: tc.colors[0] },
            areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: 'rgba(8,145,178, 0.15)' }, { offset: 1, color: 'rgba(8,145,178, 0.01)' }
            ])},
            symbol: 'none'
        }]
    });
}

function renderLatencyTrend() {
    const m = reportData.metrics || {};
    const timestamps = m.timestamps || [];
    const isSmooth = chartTypes.latTrend === 'smooth';
    
    if (!timestamps.length) return;
    
    const timeLabels = timestamps.map(ts => ts.substring(11, 19));
    const chart = echarts.init(document.getElementById('latencyTrendChart'));
    
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.96)',
            borderColor: 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 12,
            padding: [12, 16],
            textStyle: { fontSize: 11, color: '#475569' },
            extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
            formatter: function(params) {
                let h = '<div style="font-size:11.5px;color:#1e293b;margin-bottom:6px;font-weight:600">' + timeLabels[params[0].dataIndex] + '</div>';
                params.forEach(function(item) {
                    h += item.marker + ' ' + item.seriesName + ': <strong>' + item.value.toFixed(1) + '</strong>ms<br/>';
                });
                return h;
            }
        },
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(8,145,178, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: [
            { type: 'value', position: 'left', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
            { type: 'value', position: 'right', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } } }
        ],
        series: [
            { name: 'P50', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p50 || [], yAxisIndex: 0, lineStyle: { width: 2, color: '#0891b2' }, itemStyle: { color: '#0891b2' }, symbol: 'none' },
            { name: 'P90', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p90 || [], yAxisIndex: 0, lineStyle: { width: 2, color: '#d97706' }, itemStyle: { color: '#d97706' }, symbol: 'none' },
            { name: 'P95', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p95 || [], yAxisIndex: 0, lineStyle: { width: 2, color: '#d97706' }, itemStyle: { color: '#d97706' }, symbol: 'none' },
            { name: 'P99', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p99 || [], yAxisIndex: 0, lineStyle: { width: 2, color: '#8b5cf6' }, itemStyle: { color: '#8b5cf6' }, symbol: 'none' }
        ]
    });
}

function renderNodeCharts() {
    const nodes = reportData.node_metrics || [];
    nodes.forEach(function(node, idx) {
        const el = document.getElementById('nodeChart' + idx);
        if (!el) return;
        
        const timestamps = node.timestamps || [];
        const qpsData = node.ts_qps || [];
        
        if (!timestamps.length || !qpsData.length) return;
        
        const timeLabels = timestamps.map(ts => ts.substring(11, 19));
        const isSmooth = chartTypes['node-' + idx] === 'smooth';
        
        const chart = echarts.init(el);
        chart.setOption({
            backgroundColor: tc.bg,
            tooltip: {
                trigger: 'axis',
                confine: true,
                backgroundColor: 'rgba(255,255,255,0.96)',
                borderColor: 'rgba(148,163,184,0.2)',
                borderWidth: 1,
                borderRadius: 12,
                padding: [12, 16],
                textStyle: { fontSize: 11, color: '#475569' },
                extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
                formatter: function(params) {
                    let h = '<div style="font-size:11.5px;color:#1e293b;margin-bottom:6px;font-weight:600">' + timeLabels[params[0].dataIndex] + '</div>';
                    params.forEach(function(item) {
                        h += item.marker + ' ' + item.seriesName + ': <strong>' + item.value.toFixed(1) + '</strong><br/>';
                    });
                    return h;
                }
            },
            grid: { top: 28, right: 48, bottom: 36, left: 48 },
            dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(8,145,178, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
            xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
            yAxis: [
                { type: 'value', position: 'left', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(1) } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
                { type: 'value', position: 'right', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } } }
            ],
            series: [
                { name: 'QPS', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: qpsData, yAxisIndex: 0, lineStyle: { width: 2, color: tc.colors[0] }, itemStyle: { color: tc.colors[0] }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: 'rgba(8,145,178, 0.12)' }, { offset: 1, color: 'rgba(8,145,178, 0.01)' }
                ])}, symbol: 'none' },
                { name: 'P50', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: node.ts_p50 || [], yAxisIndex: 1, lineStyle: { width: 2, color: '#0891b2' }, itemStyle: { color: '#0891b2' }, symbol: 'none' },
                { name: 'P90', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: node.ts_p90 || [], yAxisIndex: 1, lineStyle: { width: 2, color: '#d97706' }, itemStyle: { color: '#d97706' }, symbol: 'none' },
                { name: 'P95', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: node.ts_p95 || [], yAxisIndex: 1, lineStyle: { width: 2, color: '#d97706' }, itemStyle: { color: '#d97706' }, symbol: 'none' },
                { name: 'P99', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: node.ts_p99 || [], yAxisIndex: 1, lineStyle: { width: 2, color: '#8b5cf6' }, itemStyle: { color: '#8b5cf6' }, symbol: 'none' }
            ]
        });
    });
}

function renderErrorBreakdownChart() {
    const errors = reportData.error_breakdown || [];
    if (!errors.length) return;
    
    const chart = echarts.init(document.getElementById('errorBreakdownChart'));
    const data = errors.map(function(e) {
        return { value: e.count, name: e.code || e.status || e.error_type || 'Unknown' };
    });
    
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'item',
            confine: true,
            backgroundColor: 'rgba(255,255,255,0.97)',
            borderColor: 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.12);',
            formatter: function(p) {
                return '<div style="font-weight:600;margin-bottom:4px">' + p.name + '</div>' +
                       '<div style="font-size:13px"><strong>' + p.value.toLocaleString() + '</strong> occurrences (' + p.percent.toFixed(1) + '%)</div>';
            }
        },
        legend: { orient: 'vertical', right: 20, top: 'center' },
        series: [{
            type: 'pie',
            radius: ['45%', '70%'],
            center: ['40%', '50%'],
            data: data,
            label: { show: true, formatter: '{b}: {c}' },
            emphasis: { itemStyle: { shadowBlur: 20, shadowColor: 'rgba(0,0,0,0.15)' } }
        }]
    });
}

// Type toggle functions - unified
function updateToggleButtons(chartId) {
    const card = document.getElementById(chartId)?.closest('.chart-card');
    if (!card) return;
    const btns = card.querySelectorAll('.type-btn');
    // Find the correct toggle based on context
}

// Initialize on load
window.addEventListener('DOMContentLoaded', initCharts);
window.addEventListener('resize', function() {
    echarts.getInstanceByDom(document.getElementById('overviewChart'))?.resize();
    echarts.getInstanceByDom(document.getElementById('errorRateChart'))?.resize();
    echarts.getInstanceByDom(document.getElementById('latencyChart'))?.resize();
    echarts.getInstanceByDom(document.getElementById('qpsChart'))?.resize();
    echarts.getInstanceByDom(document.getElementById('latencyTrendChart'))?.resize();
    if (reportData.node_metrics) {
        reportData.node_metrics.forEach(function(_, idx) {
            echarts.getInstanceByDom(document.getElementById('nodeChart' + idx))?.resize();
        });
    }
});
</script>
</body>
</html>`))

type EnhancedReportContext struct {
	Metrics        *EnhancedMetrics         `json:"metrics"`
	NodeMetrics    []EnhancedNodeMetric     `json:"node_metrics"`
	ErrorBreakdown []map[string]interface{} `json:"error_breakdown"`
	Metadata       *EnhancedMetadata        `json:"metadata"`
	JSONData       template.JS              `json:"-"`
}

type EnhancedMetrics struct {
	TotalRequests float64 `json:"total_reqs"`
	SuccessCount  float64 `json:"success_reqs"`
	FailCount     float64 `json:"failed_reqs"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorRate     float64 `json:"error_rate"`
	DurationS     float64 `json:"duration_s"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P90LatencyMs  float64 `json:"p90_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
	MinLatencyMs  float64 `json:"min_latency_ms"`
	TTFBMs        float64 `json:"ttfb_ms"`
	PeakQPS       float64 `json:"peak_qps"`
	AvgQPS        float64 `json:"avg_qps"`
	Throughput    float64 `json:"throughput"`
	WorkerCount   int     `json:"worker_count"`

	Timestamps []string  `json:"timestamps,omitempty"`
	TsQPS      []float64 `json:"ts_qps,omitempty"`
	TsTotal    []float64 `json:"ts_total,omitempty"`
	TsSuccess  []float64 `json:"ts_success,omitempty"`
	TsFail     []float64 `json:"ts_fail,omitempty"`
	TsP50      []float64 `json:"ts_p50,omitempty"`
	TsP90      []float64 `json:"ts_p90,omitempty"`
	TsP95      []float64 `json:"ts_p95,omitempty"`
	TsP99      []float64 `json:"ts_p99,omitempty"`

	AvgLatencyS string `json:"avg_latency_s"`
	P50LatencyS string `json:"p50_latency_s"`
	P90LatencyS string `json:"p90_latency_s"`
	P95LatencyS string `json:"p95_latency_s"`
	P99LatencyS string `json:"p99_latency_s"`
}

type EnhancedNodeMetric struct {
	Name    string              `json:"name"`
	Summary EnhancedNodeSummary `json:"summary"`

	Timestamps []string  `json:"timestamps,omitempty"`
	TsQPS      []float64 `json:"ts_qps,omitempty"`
	TsTotal    []float64 `json:"ts_total,omitempty"`
	TsSuccess  []float64 `json:"ts_success,omitempty"`
	TsFail     []float64 `json:"ts_fail,omitempty"`
	TsP50      []float64 `json:"ts_p50,omitempty"`
	TsP90      []float64 `json:"ts_p90,omitempty"`
	TsP95      []float64 `json:"ts_p95,omitempty"`
	TsP99      []float64 `json:"ts_p99,omitempty"`
}

type EnhancedNodeSummary struct {
	TotalRequests float64 `json:"total_requests"`
	SuccessCount  float64 `json:"success_count"`
	FailCount     float64 `json:"fail_count"`
	SuccessRate   float64 `json:"success_rate"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P90LatencyMs  float64 `json:"p90_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	AvgQPS        float64 `json:"avg_qps"`
	PeakQPS       float64 `json:"peak_qps"`
	TTFBMs        float64 `json:"ttfb_ms"`
}

type EnhancedMetadata struct {
	RunMode         string    `json:"run_mode"`
	Status          string    `json:"status"`
	StatusLabel     string    `json:"status_label"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	PlannedDuration int       `json:"planned_duration"`
	PlannedCount    int       `json:"planned_count"`
}

func GenerateEnhancedHTML(detailJSON string) (string, error) {
	var detail runner.ReportDetail
	if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
		return "", fmt.Errorf("failed to parse report detail: %w", err)
	}

	ctx := buildEnhancedContext(&detail)

	jsonBytes, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal context: %w", err)
	}
	ctx.JSONData = template.JS(jsonBytes)

	var buf strings.Builder
	if err := enhancedReportTemplate.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func buildEnhancedContext(detail *runner.ReportDetail) *EnhancedReportContext {
	ctx := &EnhancedReportContext{
		Metadata: &EnhancedMetadata{
			RunMode:         detail.Metadata.RunMode,
			Status:          detail.Metadata.Status,
			StartedAt:       detail.Metadata.StartedAt,
			FinishedAt:      detail.Metadata.FinishedAt,
			PlannedDuration: int(detail.Metadata.PlannedDuration),
			PlannedCount:    int(detail.Metadata.PlannedCount),
		},
	}

	gs := detail.GlobalSummary
	ctx.Metrics = &EnhancedMetrics{
		TotalRequests: float64(gs.TotalRequests),
		SuccessCount:  float64(gs.SuccessCount),
		FailCount:     float64(gs.FailCount),
		SuccessRate:   gs.SuccessRate,
		DurationS:     detail.Metadata.DurationSec,
		AvgLatencyMs:  gs.AvgLatencyMs,
		P50LatencyMs:  gs.P50LatencyMs,
		P90LatencyMs:  gs.P90LatencyMs,
		P95LatencyMs:  gs.P95LatencyMs,
		P99LatencyMs:  gs.P99LatencyMs,
		MinLatencyMs:  gs.MinLatencyMs,
		TTFBMs:        gs.TTFB,
		PeakQPS:       gs.PeakQPS,
		Throughput:    gs.Throughput,
		WorkerCount:   detail.Metadata.WorkerCount,

		AvgLatencyS: fmt.Sprintf("%v", gs.AvgLatencyMs/1000),
		P50LatencyS: fmt.Sprintf("%v", gs.P50LatencyMs/1000),
		P90LatencyS: fmt.Sprintf("%v", gs.P90LatencyMs/1000),
		P95LatencyS: fmt.Sprintf("%v", gs.P95LatencyMs/1000),
		P99LatencyS: fmt.Sprintf("%v", gs.P99LatencyMs/1000),
	}

	totalReqs := float64(gs.TotalRequests)
	if totalReqs > 0 {
		ctx.Metrics.ErrorRate = (float64(gs.FailCount) / totalReqs) * 100
	}
	if detail.Metadata.DurationSec > 0 {
		ctx.Metrics.AvgQPS = totalReqs / detail.Metadata.DurationSec
	}

	for _, ts := range detail.GlobalTimeSeries {
		ctx.Metrics.Timestamps = append(ctx.Metrics.Timestamps, ts.Timestamp.Format("2006-01-02 15:04:05"))
		ctx.Metrics.TsQPS = append(ctx.Metrics.TsQPS, ts.QPS)
		ctx.Metrics.TsTotal = append(ctx.Metrics.TsTotal, float64(ts.TotalRequests))
		ctx.Metrics.TsSuccess = append(ctx.Metrics.TsSuccess, float64(ts.SuccessCount))
		ctx.Metrics.TsFail = append(ctx.Metrics.TsFail, float64(ts.FailCount))
		ctx.Metrics.TsP50 = append(ctx.Metrics.TsP50, ts.P50LatencyMs)
		ctx.Metrics.TsP90 = append(ctx.Metrics.TsP90, ts.P90LatencyMs)
		ctx.Metrics.TsP95 = append(ctx.Metrics.TsP95, ts.P95LatencyMs)
		ctx.Metrics.TsP99 = append(ctx.Metrics.TsP99, ts.P99LatencyMs)
	}

	for _, e := range detail.ErrorSummary {
		item := map[string]interface{}{
			"error_type": e.ErrorType,
			"message":    e.Message,
			"count":      e.Count,
		}
		ctx.ErrorBreakdown = append(ctx.ErrorBreakdown, item)
	}

	for _, nm := range detail.NodeMetrics {
		node := EnhancedNodeMetric{
			Name: nm.NodeName,
			Summary: EnhancedNodeSummary{
				TotalRequests: float64(nm.Summary.TotalRequests),
				SuccessCount:  float64(nm.Summary.SuccessCount),
				FailCount:     float64(nm.Summary.FailCount),
				SuccessRate:   nm.Summary.SuccessRate,
				P50LatencyMs:  nm.Summary.P50LatencyMs,
				P90LatencyMs:  nm.Summary.P90LatencyMs,
				P95LatencyMs:  nm.Summary.P95LatencyMs,
				AvgQPS:        nm.Summary.AvgQPS,
				PeakQPS:       nm.Summary.PeakQPS,
				TTFBMs:        nm.Summary.TTFB,
			},
		}

		for _, ts := range nm.TimeSeries {
			node.Timestamps = append(node.Timestamps, ts.Timestamp.Format("2006-01-02 15:04:05"))
			node.TsQPS = append(node.TsQPS, ts.QPS)
			node.TsTotal = append(node.TsTotal, float64(ts.TotalRequests))
			node.TsSuccess = append(node.TsSuccess, float64(ts.SuccessCount))
			node.TsFail = append(node.TsFail, float64(ts.FailCount))
			node.TsP50 = append(node.TsP50, ts.P50LatencyMs)
			node.TsP90 = append(node.TsP90, ts.P90LatencyMs)
			node.TsP95 = append(node.TsP95, ts.P95LatencyMs)
			node.TsP99 = append(node.TsP99, ts.P99LatencyMs)
		}

		ctx.NodeMetrics = append(ctx.NodeMetrics, node)
	}

	statusMap := map[string]string{
		"completed": "已完成",
		"running":   "运行中",
		"failed":    "失败",
		"canceled":  "已取消",
		"pending":   "等待中",
	}
	ctx.Metadata.StatusLabel = statusMap[ctx.Metadata.Status]

	runModeMap := map[string]string{
		"duration": "按时长运行",
		"count":    "按次数运行",
	}
	if mode, ok := runModeMap[ctx.Metadata.RunMode]; ok {
		ctx.Metadata.RunMode = mode
	}

	return ctx
}
