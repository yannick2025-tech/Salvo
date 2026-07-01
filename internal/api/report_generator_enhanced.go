package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/runner"
)

//go:embed echarts.min.js
var echartsMinJS string

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
    <script>{{.EChartsJS}}</script>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-secondary: #f9fafb;
            --bg-card: #ffffff;
            --bg-tertiary: #f3f4f6;
            --bg-hover: rgba(45,212,191,0.04);
            --text-primary: #111827;
            --text-secondary: #374151;
            --text-tertiary: #6b7280;
            --border-primary: #e5e7eb;
            --border-secondary: #e5e7eb;
            --border-color: #e5e7eb;
            --border-light: #f3f4f6;
            --accent-primary: #0d9488;
            --accent-success: #1a7f37;
            --accent-danger: #cf222e;
            --accent-warning: #bf8700;
            --accent-info: #0969da;
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
        #themeToggle {
            display: flex;
            align-items: center;
            gap: 6px;
            padding: 6px 12px;
            font-size: 13px;
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-sm);
            background: transparent;
            cursor: pointer;
            transition: all 0.15s ease;
            color: var(--text-secondary);
        }
        #themeToggle:hover {
            background: var(--bg-hover);
            color: var(--text-primary);
            border-color: var(--accent-primary);
        }
        #themeToggle svg {
            width: 16px;
            height: 16px;
            flex-shrink: 0;
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
        .nodes-section .chart-card { margin-bottom: 16px; overflow: visible; }
        .nodes-section .chart-card:last-child { margin-bottom: 0; }
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
            background: rgba(45,212,191,0.1);
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
        .status-badge.success { background: rgba(26,127,55,0.15); color: #1a7f37; }
        .status-badge.partial { background: rgba(191,135,0,0.15); color: #bf8700; }
        .status-badge.failed { background: rgba(207,34,46,0.15); color: #cf222e; }
        .status-badge.running { background: rgba(13,148,136,0.15); color: #0d9488; }
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
        .node-chart-body { height: 200px; }
        .node-chart-grid { display: flex; flex-direction: column; gap: 8px; }
        .node-chart-panel { position: relative; }
        .node-chart-panel-label {
            font-size: 11px; font-weight: 600; color: var(--text-secondary);
            letter-spacing: 0.02em; margin-bottom: 2px;
            padding-left: 12px;
            display: inline-flex; align-items: center; gap: 4px;
        }
        .tooltip-wrapper.node-chart-help {
            display: inline-flex; align-items: center; justify-content: center;
            width: 16px; height: 16px;
            color: var(--text-tertiary);
            background: transparent; border: none;
            line-height: 1; cursor: help; position: relative;
        }
        .node-chart-help-icon {
            display: block; flex: 0 0 auto;
            transition: color 0.15s ease;
        }
        .tooltip-wrapper.node-chart-help:hover .node-chart-help-icon {
            color: var(--accent-primary);
        }
        /* 悬浮注释：从 ? 右侧弹出，与标签同行，箭头朝左 */
        .tooltip-wrapper.node-chart-help::before {
            content: attr(data-tooltip);
            position: absolute; visibility: hidden; opacity: 0;
            white-space: normal; width: 240px; text-align: left; line-height: 1.5;
            font-size: 11px; font-weight: 400; color: #475569;
            background: rgba(255, 255, 255, 0.96);
            border: 1px solid rgba(148, 163, 184, 0.2);
            border-radius: 8px; padding: 10px 14px;
            box-shadow: 0 4px 16px rgba(15, 23, 42, 0.12);
            bottom: auto; top: 50%;
            left: calc(100% + 10px);
            transform: translateY(-50%) translateX(-6px);
            transition: opacity 0.15s ease, transform 0.15s ease;
            z-index: 100; pointer-events: none;
        }
        .tooltip-wrapper.node-chart-help::after {
            content: ''; position: absolute; visibility: hidden; opacity: 0;
            width: 0; height: 0;
            border-style: solid; border-width: 6px;
            border-color: transparent;
            border-top-color: transparent;
            border-bottom-color: transparent;
            border-right-color: rgba(255, 255, 255, 0.96);
            bottom: auto; top: 50%;
            left: 100%;
            transform: translateY(-50%) translateX(-2px);
            transition: opacity 0.15s ease;
            z-index: 101; pointer-events: none;
        }
        .tooltip-wrapper.node-chart-help:hover::before,
        .tooltip-wrapper.node-chart-help:hover::after {
            visibility: visible; opacity: 1;
        }
        .tooltip-wrapper.node-chart-help:hover::before { transform: translateY(-50%) translateX(0); }
        .node-band-legend { margin-top: 6px; line-height: 1.5; }
        .node-qps-chart { height: 160px; }
        .node-latency-chart { height: 220px; }

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

        /* System Performance Section */
        .sys-summary-row {
            display: flex;
            gap: 12px;
            flex-wrap: wrap;
            margin-bottom: 16px;
        }
        .sys-summary-card {
            flex: 1;
            min-width: 140px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-secondary);
            border-radius: var(--radius-md);
            padding: 12px 16px;
            text-align: center;
        }
        .sys-summary-label {
            font-size: 11px;
            color: var(--text-secondary);
            margin-bottom: 4px;
            font-weight: 500;
        }
        .sys-summary-value {
            font-size: 20px;
            font-weight: 700;
            color: var(--text-primary);
            font-family: -apple-system, 'SF Mono', 'Monaco', 'Menlo', monospace;
        }
        .sys-summary-sub {
            font-size: 10px;
            color: var(--text-tertiary);
            margin-top: 2px;
        }
        .sys-charts-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 12px;
        }

        .sys-table-section {
            margin-top: 16px;
        }

        .table-header-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }

        .table-header-row h4 {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
            margin: 0;
        }

        .table-info {
            font-size: 12px;
            color: var(--text-tertiary);
        }

        .table-wrapper {
            overflow-x: auto;
            border: 1px solid var(--border-color);
            border-radius: 8px;
            background: var(--bg-card);
        }

        .data-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 13px;
        }

        .data-table thead {
            background: var(--bg-secondary);
            position: sticky;
            top: 0;
            z-index: 10;
        }

        .data-table th {
            padding: 10px 12px;
            text-align: left;
            font-weight: 600;
            color: var(--text-primary);
            border-bottom: 2px solid var(--border-color);
            white-space: nowrap;
            user-select: none;
        }

        .data-table th.sortable {
            cursor: pointer;
            transition: background-color 0.2s;
        }

        .data-table th.sortable:hover {
            background: var(--bg-hover);
        }

        .sort-icon {
            margin-left: 4px;
            opacity: 0.5;
            font-size: 11px;
        }

        .data-table td {
            padding: 8px 12px;
            border-bottom: 1px solid var(--border-light);
            color: var(--text-primary);
        }

        .data-table tbody tr:hover {
            background: var(--bg-hover);
        }

        .danger-row {
            background: rgba(239, 68, 68, 0.05) !important;
        }

        .danger-cell {
            color: var(--accent-danger) !important;
            font-weight: 600;
        }

        .pagination-controls {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 8px;
            margin-top: 16px;
        }

        .page-btn {
            padding: 6px 12px;
            font-size: 12px;
            border: 1px solid var(--border-color);
            border-radius: 6px;
            background: var(--bg-card);
            color: var(--text-primary);
            cursor: pointer;
            transition: all 0.2s;
        }

        .page-btn:hover:not(:disabled) {
            background: var(--bg-hover);
            border-color: var(--accent-info);
        }

        .page-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        .page-info {
            font-size: 12px;
            color: #6b7280;
            min-width: 100px;
            text-align: center;
        }

        html.dark-theme {
            --bg-primary: #0d1117;
            --bg-secondary: #161b22;
            --bg-card: #161b22;
            --bg-tertiary: #21262d;
            --bg-hover: #21262d;
            --text-primary: #c9d1d9;
            --text-secondary: #8b949e;
            --text-tertiary: #6e7681;
            --border-primary: #30363d;
            --border-secondary: #30363d;
            --border-color: #30363d;
            --border-light: #21262d;
            --accent-primary: #2dd4bf;
            --accent-success: #3fb950;
            --accent-danger: #f85149;
            --accent-warning: #e3b341;
            --accent-info: #58a6ff;
        }
        html.dark-theme body { background: var(--bg-primary); color: var(--text-primary); }
        html.dark-theme .container { background: var(--bg-primary); }
        html.dark-theme .metric-card, html.dark-theme .chart-card, html.dark-theme .sys-summary-card, html.dark-theme .nodes-section, html.dark-theme .charts-section { background: var(--bg-card); border-color: var(--border-color); }
        html.dark-theme .metric-label, html.dark-theme .sys-summary-label, html.dark-theme .chart-header h3, html.dark-theme h2, html.dark-theme h3, html.dark-theme h4, html.dark-theme .charts-toolbar h3, html.dark-theme .nodes-section h3 { color: var(--text-primary); }
        html.dark-theme .metric-sub, html.dark-theme .sys-summary-sub, html.dark-theme .table-info { color: var(--text-secondary); }
        html.dark-theme .table-wrapper { background: var(--bg-card); border-color: var(--border-color); }
        html.dark-theme .data-table thead { background: var(--bg-secondary); }
        html.dark-theme .data-table th { color: var(--text-primary); border-bottom-color: var(--border-color); }
        html.dark-theme .data-table td { color: var(--text-primary); border-bottom-color: var(--border-light); }
        html.dark-theme .data-table tbody tr:hover { background: var(--bg-hover); }
        html.dark-theme .page-btn { background: var(--bg-card); color: var(--text-primary); border-color: var(--border-color); }
        html.dark-theme .page-btn:hover:not(:disabled) { background: var(--bg-hover); }
        html.dark-theme .type-btn { background: transparent; color: var(--text-secondary); border-color: var(--border-color); }
        html.dark-theme .type-btn.active { background: var(--accent-primary); color: #fff; border-color: var(--accent-primary); }
        html.dark-theme .type-btn:hover:not(.active) { background: var(--bg-hover); border-color: var(--accent-primary); }
        html.dark-theme .node-badge { background: var(--bg-secondary); color: var(--text-secondary); border: 1px solid var(--border-color); }
        html.dark-theme .info-card { background: var(--bg-card); border-color: var(--border-color); }
        html.dark-theme .perf-divider { background: var(--border-color); }
        html.dark-theme .info-table td { color: var(--text-primary); border-bottom-color: var(--border-light); }
        html.dark-theme .status-badge { border: 1px solid var(--border-color); }
        html.dark-theme .page-info { color: var(--text-secondary); }
        html.dark-theme .info-banner { background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 197, 253, 0.15)); border-color: rgba(59, 130, 246, 0.3); }
        html.dark-theme .banner-text strong { color: var(--text-primary); }
        html.dark-theme .banner-text p { color: var(--text-secondary); }
        html.dark-theme .tooltip-wrapper.node-chart-help { color: var(--text-secondary); }
        html.dark-theme .tooltip-wrapper.node-chart-help::before {
            background: rgba(22, 27, 34, 0.96);
            border-color: rgba(48, 54, 61, 0.8);
            color: #e6edf3;
            box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
        }
        html.dark-theme .tooltip-wrapper.node-chart-help::after { border-right-color: rgba(22, 27, 34, 0.96); }

        @media print {
            body { background: white; color: black; }
            .container { max-width: 100%; padding: 0; }
            .page-header, .metrics-row, .charts-row, .nodes-section, .sys-table-section { page-break-inside: avoid; }
            .chart-card { page-break-inside: avoid; border: 1px solid #ddd; margin-bottom: 16px; }
            .chart-body canvas { max-width: 100% !important; height: auto !important; }
            .data-table { font-size: 11px; }
            .pagination-controls, .chart-type-toggle, .btn-back, button { display: none !important; }
            .danger-cell { color: black !important; font-weight: bold; background: #f0f0f0; }
            .danger-row { background: #f5f5f5 !important; }
            h2, h3, h4 { page-break-after: avoid; }
        }
    </style>
</head>
<body>
<div class="container">
    <div class="page-header">
        <h2>测试报告</h2>
        <button id="themeToggle" onclick="toggleTheme()" title="切换主题">
            <svg id="themeIconSun" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="display:none">
                <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
            <svg id="themeIconMoon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
            <span id="themeText">深色</span>
        </button>
    </div>

    {{if .Metrics}}
    <!-- Metrics Row -->
    <div class="metrics-row">
        <div class="metric-card">
            <div class="metric-label">成功率</div>
            <div class="metric-value" style="color: {{if gt .Metrics.SuccessRate 90.0}}#1a7f37{{else if gt .Metrics.SuccessRate 70.0}}#bf8700{{else}}#cf222e{{end}}">{{formatSuccessRate .Metrics.SuccessRate}}</div>
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
            <div class="node-chart-grid">
                <div class="node-chart-panel">
                    <div class="node-chart-panel-label">QPS</div>
                    <div class="chart-body node-chart-body node-qps-chart" id="nodeQpsChart{{$idx}}"></div>
                </div>
                <div class="node-chart-panel">
                    <div class="node-chart-panel-label">延迟百分位带<span class="tooltip-wrapper node-chart-help" data-tooltip="百分位带：P50-P90 绿带=常规波动，P90-P95 黄带=注意，P95-P99 橙带=尾部警告。带越厚=延迟波动越大，带越薄=稳定。P50(底线)与P99(顶线)为分布边界。"><svg class="node-chart-help-icon" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" stroke-width="1.2"/><circle cx="8" cy="4.6" r="0.9" fill="currentColor"/><path d="M8 7v5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg></span></div>
                    <div class="chart-body node-chart-body node-latency-chart" id="nodeLatencyChart{{$idx}}"></div>
                </div>
            </div>
            <div class="chart-tip node-band-legend">
                色带含义：P50-P90 绿带=常规波动，P90-P95 黄带=注意，P95-P99 橙带=尾部警告。带越厚=延迟波动越大。
            </div>
            <div class="chart-type-toggle">
                <button class="type-btn active" onclick="switchChartType('node-{{$idx}}', 'smooth')">平滑</button>
                <button class="type-btn" onclick="switchChartType('node-{{$idx}}', 'step')">阶梯</button>
            </div>
        </div>
        {{end}}
    </section>
    {{end}}

    {{end}}

    {{if .SystemMetrics}}
    <!-- System Performance Analysis Section -->
    <section class="nodes-section">
        <h3>系统性能分析</h3>
        <div class="sys-summary-row">
            <div class="sys-summary-card" style="border-color: #0d9488;">
                <div class="sys-summary-label">Goroutine 峰值</div>
                <div class="sys-summary-value">{{.SystemMetrics.Summary.GoroutineMax}}</div>
                <div class="sys-summary-sub">平均 {{printf "%.0f" .SystemMetrics.Summary.GoroutineAvg}}</div>
            </div>
            <div class="sys-summary-card" style="border-color: #58a6ff;">
                <div class="sys-summary-label">Heap 峰值</div>
                <div class="sys-summary-value">{{printf "%.3f" .SystemMetrics.Summary.HeapAllocMaxMB}} MB</div>
                <div class="sys-summary-sub">平均 {{printf "%.3f" .SystemMetrics.Summary.HeapAllocAvgMB}} MB</div>
            </div>
            <div class="sys-summary-card" style="border-color: #bf8700;">
                <div class="sys-summary-label">CPU 峰值</div>
                <div class="sys-summary-value" style="color: {{if gt .SystemMetrics.Summary.CPUMax 90.0}}#cf222e{{else if gt .SystemMetrics.Summary.CPUMax 70.0}}#bf8700{{else}}#bf8700{{end}}">{{printf "%.3f" .SystemMetrics.Summary.CPUMax}}%</div>
                <div class="sys-summary-sub">平均 {{printf "%.3f" .SystemMetrics.Summary.CPUAvg}}%</div>
            </div>
            <div class="sys-summary-card" style="border-color: #cf222e;">
                <div class="sys-summary-label">GC 暂停</div>
                <div class="sys-summary-value">{{printf "%.3f" .SystemMetrics.Summary.GCPauseTotalMs}} ms</div>
                <div class="sys-summary-sub">共 {{.SystemMetrics.Summary.GCCount}} 次</div>
            </div>
            <div class="sys-summary-card" style="border-color: #cf222e;">
                <div class="sys-summary-label">任务等待 P99 峰值</div>
                <div class="sys-summary-value">{{printf "%.3f" .SystemMetrics.Summary.TaskWaitP99MaxMs}} ms</div>
                <div class="sys-summary-sub">平均 {{printf "%.3f" .SystemMetrics.Summary.TaskWaitAvgMs}} ms</div>
            </div>
            <div class="sys-summary-card" style="border-color: #0969da;">
                <div class="sys-summary-label">Pending Queue 峰值</div>
                <div class="sys-summary-value">{{.SystemMetrics.Summary.PendingQueueMax}}</div>
                <div class="sys-summary-sub">平均 {{printf "%.0f" .SystemMetrics.Summary.PendingQueueAvg}}</div>
            </div>
            <div class="sys-summary-card" style="border-color: #1a7f37;">
                <div class="sys-summary-label">Active Workers 峰值</div>
                <div class="sys-summary-value">{{.SystemMetrics.Summary.ActiveWorkersMax}}</div>
                <div class="sys-summary-sub">平均 {{printf "%.0f" .SystemMetrics.Summary.ActiveWorkersAvg}}</div>
            </div>
        </div>
        {{if gt (len .SystemMetrics.TimeSeries) 1}}
        <div class="sys-charts-row">
            <div class="chart-card">
                <div class="chart-header"><h3>Goroutine 趋势</h3></div>
                <div class="chart-body" id="sysGoroutineChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('sysGoroutine', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('sysGoroutine', 'step')">阶梯</button>
                </div>
            </div>
            <div class="chart-card">
                <div class="chart-header"><h3>Heap 内存趋势</h3></div>
                <div class="chart-body" id="sysHeapChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('sysHeap', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('sysHeap', 'step')">阶梯</button>
                </div>
            </div>
            <div class="chart-card">
                <div class="chart-header"><h3>CPU 使用率趋势</h3></div>
                <div class="chart-body" id="sysCpuChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('sysCpu', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('sysCpu', 'step')">阶梯</button>
                </div>
            </div>
            <div class="chart-card">
                <div class="chart-header"><h3>任务等待时间趋势</h3></div>
                <div class="chart-body" id="sysTaskWaitChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('sysTaskWait', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('sysTaskWait', 'step')">阶梯</button>
                </div>
            </div>
            <div class="chart-card">
                <div class="chart-header"><h3>Pending Queue 趋势</h3></div>
                <div class="chart-body" id="sysQueueChart"></div>
                <div class="chart-type-toggle">
                    <button class="type-btn active" onclick="switchChartType('sysQueue', 'smooth')">平滑</button>
                    <button class="type-btn" onclick="switchChartType('sysQueue', 'step')">阶梯</button>
                </div>
            </div>
        </div>
        {{end}}
    </section>
    {{end}}

    {{if gt (len .SystemMetrics.TimeSeries) 0}}
    <div class="sys-table-section">
        <div class="table-header-row">
            <h4>系统指标详细数据</h4>
            <div class="table-info">共 {{len .SystemMetrics.TimeSeries}} 条记录</div>
        </div>
        <div class="table-wrapper">
            <table class="data-table" id="sysMetricsTable">
                <thead>
                    <tr>
                        <th class="sortable" onclick="toggleSort('timestamp')">时间戳 <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('goroutine_count')">Goroutines <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('heap_alloc_mb')">Heap (MB) <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('cpu_percent')">CPU (%) <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('gc_pause_last_ms')">GC 暂停 (ms) <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('active_workers')">Workers <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('pending_queue_len')">队列长度 <span class="sort-icon">⇅</span></th>
                        <th class="sortable" onclick="toggleSort('task_wait_p99_ms')">Wait P99 (ms) <span class="sort-icon">⇅</span></th>
                    </tr>
                </thead>
                <tbody id="sysMetricsTableBody"></tbody>
            </table>
        </div>
        <div class="pagination-controls" id="tablePagination" style="display: none;">
            <button class="page-btn" onclick="goToPage(1)">首页</button>
            <button class="page-btn" onclick="prevPage()">上一页</button>
            <span class="page-info" id="pageInfo"></span>
            <button class="page-btn" onclick="nextPage()">下一页</button>
            <button class="page-btn" onclick="goToPage(totalPages)">末页</button>
        </div>
    </div>
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
    bg: '#0d1117',
    textColor: '#c9d1d9',
    lineColor: '#30363d',
    // 主色系：蓝绿/绿/黄/橙/红/蓝/灰
    colors: ['#2dd4bf', '#3fb950', '#e3b341', '#f0883e', '#f85149', '#58a6ff', '#94a3b8'],
    // 延迟曲线语义色：P50 绿 → P90 黄 → P95 橙 → P99 红
    latencyColors: ['#3fb950', '#e3b341', '#f0883e', '#f85149'],
    dangerColor: '#f85149'
};

let errorRateType = 'smooth';
let qpsType = 'smooth';
let latTrendType = 'smooth';
let nodeType = 'smooth';

const chartTypes = { errorRate: 'smooth', qpsTrend: 'smooth', latTrend: 'smooth', sysGoroutine: 'smooth', sysHeap: 'smooth', sysCpu: 'smooth', sysTaskWait: 'smooth', sysQueue: 'smooth' };

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
    else if (chartId === 'sysGoroutine') { updateSysToggleButtons('sysGoroutineChart'); renderSysGoroutineChart(); }
    else if (chartId === 'sysHeap') { updateSysToggleButtons('sysHeapChart'); renderSysHeapChart(); }
    else if (chartId === 'sysCpu') { updateSysToggleButtons('sysCpuChart'); renderSysCpuChart(); }
    else if (chartId === 'sysTaskWait') { updateSysToggleButtons('sysTaskWaitChart'); renderSysTaskWaitChart(); }
    else if (chartId === 'sysQueue') { updateSysToggleButtons('sysQueueChart'); renderSysQueueChart(); }
}

function updateSysToggleButtons(chartId) {
    var el = document.getElementById(chartId);
    if (!el) return;
    var card = el.closest('.chart-card');
    if (!card) return;
    var btns = card.querySelectorAll('.type-btn');
    btns.forEach(function(btn) {
        btn.classList.toggle('active', btn.textContent.trim().toLowerCase() === chartTypes[chartId]);
    });
}

function renderSysGoroutineChart() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length < 2) return;
    var el = document.getElementById('sysGoroutineChart');
    if (!el) return;
    var chart = echarts.init(el);
    var isSmooth = chartTypes.sysGoroutine === 'smooth';
    var ts = sm.time_series;
    var labels = ts.map(function(s) { return s.timestamp.substring(11, 19); });
    var data = ts.map(function(s) { return s.goroutine_count; });
    chart.setOption({
        backgroundColor: tc.bg,
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        series: [{ name: 'Goroutines', data: data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[0], width: 2 }, itemStyle: { color: tc.colors[0] }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 10000, lineStyle: { color: tc.colors[2] }, label: { formatter: '10K', color: tc.colors[2], fontSize: 10 } }, { yAxis: 50000, lineStyle: { color: tc.dangerColor }, label: { formatter: '50K', color: tc.dangerColor, fontSize: 10 } }] } }],
        tooltip: {
            trigger: 'axis',
            confine: true,
            formatter: function(params) {
                return params[0].axisValue + '<br/>' +
                       params[0].marker + ' ' + params[0].seriesName + ': <strong>' + Number(params[0].value).toFixed(3) + '</strong>';
            }
        },
        legend: { data: ['Goroutines'], textStyle: { color: tc.textColor }, top: 0 },
    }, true);
}

function renderSysHeapChart() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length < 2) return;
    var el = document.getElementById('sysHeapChart');
    if (!el) return;
    var chart = echarts.init(el);
    var isSmooth = chartTypes.sysHeap === 'smooth';
    var ts = sm.time_series;
    var labels = ts.map(function(s) { return s.timestamp.substring(11, 19); });
    var allocData = ts.map(function(s) { return s.heap_alloc_mb; });
    var sysData = ts.map(function(s) { return s.heap_sys_mb; });
    chart.setOption({
        backgroundColor: tc.bg,
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}MB' } },
        series: [
            { name: 'HeapAlloc', data: allocData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[0], width: 2 }, itemStyle: { color: tc.colors[0] } },
            { name: 'HeapSys', data: sysData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[5], width: 2, type: 'dashed' }, itemStyle: { color: tc.colors[5] } },
        ],
        tooltip: {
            trigger: 'axis',
            confine: true,
            formatter: function(params) {
                return params[0].axisValue + '<br/>' +
                       params[0].marker + ' HeapAlloc: <strong>' + Number(params[0].value).toFixed(3) + ' MB</strong><br/>' +
                       params[1].marker + ' HeapSys: <strong>' + Number(params[1].value).toFixed(3) + ' MB</strong>';
            }
        },
        legend: { data: ['HeapAlloc', 'HeapSys'], textStyle: { color: tc.textColor }, top: 0 },
    }, true);
}

function renderSysCpuChart() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length < 2) return;
    var el = document.getElementById('sysCpuChart');
    if (!el) return;
    var chart = echarts.init(el);
    var isSmooth = chartTypes.sysCpu === 'smooth';
    var ts = sm.time_series;
    var labels = ts.map(function(s) { return s.timestamp.substring(11, 19); });
    var data = ts.map(function(s) { return s.cpu_percent; });
    chart.setOption({
        backgroundColor: tc.bg,
        grid: { top: 30, right: 60, bottom: 50, left: 50 },
        xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', min: 0, max: 100, axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}%' } },
        series: [{ name: 'CPU', data: data, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[5], width: 2 }, itemStyle: { color: tc.colors[5] }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(88,166,255,0.2)' }, { offset: 1, color: 'rgba(88,166,255,0.01)' }]) }, markLine: { silent: true, lineStyle: { type: 'dashed' }, data: [{ yAxis: 70, lineStyle: { color: tc.colors[5] }, label: { formatter: '70%', color: tc.colors[5], fontSize: 10, position: 'end' } }, { yAxis: 90, lineStyle: { color: tc.dangerColor }, label: { formatter: '90%', color: tc.dangerColor, fontSize: 10, position: 'end' } }] } }],
        tooltip: {
            trigger: 'axis',
            confine: true,
            formatter: function(params) {
                return params[0].axisValue + '<br/>' +
                       params[0].marker + ' CPU: <strong>' + Number(params[0].value).toFixed(3) + '%</strong>';
            }
        },
        legend: { data: ['CPU'], textStyle: { color: tc.textColor }, top: 0 },
    }, true);
}

function renderSysTaskWaitChart() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length < 2) return;
    var el = document.getElementById('sysTaskWaitChart');
    if (!el) return;
    var chart = echarts.init(el);
    var isSmooth = chartTypes.sysTaskWait === 'smooth';
    var ts = sm.time_series;
    var labels = ts.map(function(s) { return s.timestamp.substring(11, 19); });
    var p50 = ts.map(function(s) { return s.task_wait_p50_ms; });
    var p95 = ts.map(function(s) { return s.task_wait_p95_ms; });
    var p99 = ts.map(function(s) { return s.task_wait_p99_ms; });
    chart.setOption({
        backgroundColor: tc.bg,
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}ms' } },
        series: [
            { name: 'P50', data: p50, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.latencyColors[0], width: 2 }, itemStyle: { color: tc.latencyColors[0] } },
            { name: 'P95', data: p95, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.latencyColors[2], width: 2 }, itemStyle: { color: tc.latencyColors[2] } },
            { name: 'P99', data: p99, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.dangerColor, width: 2 }, itemStyle: { color: tc.dangerColor } },
        ],
        tooltip: {
            trigger: 'axis',
            confine: true,
            formatter: function(params) {
                return params[0].axisValue + '<br/>' +
                       params[0].marker + ' P50: <strong>' + Number(params[0].value).toFixed(3) + ' ms</strong><br/>' +
                       params[1].marker + ' P95: <strong>' + Number(params[1].value).toFixed(3) + ' ms</strong><br/>' +
                       params[2].marker + ' P99: <strong>' + Number(params[2].value).toFixed(3) + ' ms</strong>';
            }
        },
        legend: { data: ['P50', 'P95', 'P99'], textStyle: { color: tc.textColor }, top: 0 },
    }, true);
}

function renderSysQueueChart() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length < 2) return;
    var el = document.getElementById('sysQueueChart');
    if (!el) return;
    var chart = echarts.init(el);
    var isSmooth = chartTypes.sysQueue === 'smooth';
    var ts = sm.time_series;
    var labels = ts.map(function(s) { return s.timestamp.substring(11, 19); });
    var queueData = ts.map(function(s) { return s.pending_queue_len; });
    // Track historical max for y-axis scaling
    var maxVal = 0;
    queueData.forEach(function(v) { if (v > maxVal) maxVal = v; });
    if (maxVal < 10) maxVal = 10;
    chart.setOption({
        backgroundColor: tc.bg,
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', min: 0, max: maxVal, axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        series: [
            { name: 'Pending Queue', data: queueData, type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', symbol: 'none', lineStyle: { color: tc.colors[6], width: 2 }, itemStyle: { color: tc.colors[6] }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(148,163,184,0.25)' }, { offset: 1, color: 'rgba(148,163,184,0.02)' }]) } },
        ],
        tooltip: {
            trigger: 'axis',
            confine: true,
            formatter: function(params) {
                return params[0].axisValue + '<br/>' +
                       params[0].marker + ' Pending Queue: <strong>' + Number(params[0].value) + '</strong>';
            }
        },
        legend: { data: ['Pending Queue'], textStyle: { color: tc.textColor }, top: 0 },
    }, true);
}

// System Metrics Table Functions
let tableSortKey = 'timestamp';
let tableSortOrder = 'asc';
let currentPage = 1;
let totalPages = 1;
const pageSize = 20;
let tableData = [];

function initSysMetricsTable() {
    var sm = reportData.system_metrics;
    if (!sm || !sm.time_series || sm.time_series.length === 0) return;
    tableData = sm.time_series.slice();
    renderTablePage();
    updatePagination();
}

function formatTimestamp(ts) {
    if (!ts) return '-';
    try {
        var d = new Date(ts);
        if (isNaN(d.getTime())) return ts;
        return d.toLocaleString('zh-CN', {
            year: 'numeric', month: '2-digit', day: '2-digit',
            hour: '2-digit', minute: '2-digit', second: '2-digit'
        });
    } catch (e) {
        return ts;
    }
}

function isDangerRow(row) {
    return (row.goroutine_count > 50000) ||
           (row.heap_alloc_mb > 500) ||
           (row.cpu_percent > 90) ||
           (row.gc_pause_last_ms > 10) ||
           (row.pending_queue_len > 100) ||
           (row.task_wait_p99_ms > 100);
}

function getSortedData() {
    if (!tableSortKey) return tableData;
    var key = tableSortKey;
    var order = tableSortOrder === 'asc' ? 1 : -1;
    return tableData.slice().sort(function(a, b) {
        var valA = a[key] !== undefined ? a[key] : 0;
        var valB = b[key] !== undefined ? b[key] : 0;
        if (typeof valA === 'string' && typeof valB === 'string') {
            return order * valA.localeCompare(valB);
        }
        return order * (valA - valB);
    });
}

function toggleSort(key) {
    if (tableSortKey === key) {
        tableSortOrder = tableSortOrder === 'asc' ? 'desc' : 'asc';
    } else {
        tableSortKey = key;
        tableSortOrder = 'asc';
    }
    currentPage = 1;
    renderTablePage();
    updatePagination();
    updateSortIcons();
}

function updateSortIcons() {
    var headers = document.querySelectorAll('#sysMetricsTable th.sortable');
    headers.forEach(function(th) {
        var icon = th.querySelector('.sort-icon');
        if (!icon) return;
        var col = th.getAttribute('onclick').match(/toggleSort\('(.+?)'\)/)[1];
        if (col === tableSortKey) {
            icon.textContent = tableSortOrder === 'asc' ? '↑' : '↓';
        } else {
            icon.textContent = '⇅';
        }
    });
}

function getTotalPages() {
    return Math.ceil(tableData.length / pageSize);
}

function renderTablePage() {
    var tbody = document.getElementById('sysMetricsTableBody');
    if (!tbody) return;
    var sorted = getSortedData();
    var start = (currentPage - 1) * pageSize;
    var pageData = sorted.slice(start, start + pageSize);
    var html = '';
    pageData.forEach(function(row) {
        var dangerClass = isDangerRow(row) ? 'danger-row' : '';
        html += '<tr class="' + dangerClass + '">';
        html += '<td>' + formatTimestamp(row.timestamp) + '</td>';
        html += '<td class="' + (row.goroutine_count > 50000 ? 'danger-cell' : '') + '">' + Number(row.goroutine_count || 0).toLocaleString() + '</td>';
        html += '<td class="' + (row.heap_alloc_mb > 500 ? 'danger-cell' : '') + '">' + (row.heap_alloc_mb || 0).toFixed(1) + '</td>';
        html += '<td class="' + (row.cpu_percent > 90 ? 'danger-cell' : '') + '">' + (row.cpu_percent || 0).toFixed(1) + '</td>';
        html += '<td class="' + (row.gc_pause_last_ms > 10 ? 'danger-cell' : '') + '">' + (row.gc_pause_last_ms || 0).toFixed(1) + '</td>';
        html += '<td>' + (row.active_workers || 0) + '</td>';
        html += '<td class="' + (row.pending_queue_len > 100 ? 'danger-cell' : '') + '">' + (row.pending_queue_len || 0) + '</td>';
        html += '<td class="' + (row.task_wait_p99_ms > 100 ? 'danger-cell' : '') + '">' + (row.task_wait_p99_ms || 0).toFixed(1) + '</td>';
        html += '</tr>';
    });
    tbody.innerHTML = html;
}

function updatePagination() {
    totalPages = getTotalPages();
    var pagination = document.getElementById('tablePagination');
    var pageInfo = document.getElementById('pageInfo');
    if (!pagination || !pageInfo) return;

    if (totalPages <= 1) {
        pagination.style.display = 'none';
    } else {
        pagination.style.display = 'flex';
        pageInfo.textContent = '第 ' + currentPage + ' / ' + totalPages + ' 页';
    }
}

function goToPage(page) {
    totalPages = getTotalPages();
    if (page < 1) page = 1;
    if (page > totalPages) page = totalPages;
    currentPage = page;
    renderTablePage();
    updatePagination();
}

function prevPage() {
    goToPage(currentPage - 1);
}

function nextPage() {
    goToPage(currentPage + 1);
}

function updateNodeToggleButtons(chartId) {
    const idx = parseInt(chartId.slice(5));
    const el = document.getElementById('nodeQpsChart' + idx) || document.getElementById('nodeLatencyChart' + idx);
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
    {{if .SystemMetrics}}
    renderSysGoroutineChart();
    renderSysHeapChart();
    renderSysCpuChart();
    renderSysTaskWaitChart();
    renderSysQueueChart();
    initSysMetricsTable();
    {{end}}
}

function renderOverviewChart() {
    const m = reportData.metrics || {};
    const chart = echarts.init(document.getElementById('overviewChart'));

    const total = Number(m.total_reqs || 0);
    const success = Number(m.success_reqs || 0);
    const failed = Number(m.failed_reqs || 0);

    const isDark = document.documentElement.classList.contains('dark-theme');

    chart.setOption({
        backgroundColor: tc.bg,
        color: [tc.colors[1], tc.colors[4]],
        tooltip: {
            trigger: 'item',
            confine: true,
            backgroundColor: isDark ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
            borderColor: isDark ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: isDark ? '#e6edf3' : '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.12);',
            formatter: function(p) {
                const pct = ((p.value / Math.max(total, 1)) * 100).toFixed(3);
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
                    fill: tc.textColor,
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
                const pct = (val / Math.max(total, 1) * 100).toFixed(3);
                return '{name|' + name + '}   {val|' + pct + '%}';
            },
            textStyle: {
                rich: {
                    name: { color: tc.textColor, fontWeight: 500, fontSize: 13 },
                    val: { color: tc.lineColor, fontSize: 12, fontWeight: 400 }
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
                    { offset: 0, color: isDark ? '#3fb950' : '#1a7f37' }, { offset: 1, color: isDark ? '#46c759' : '#15803d' }
                ])}},
                { value: failed, name: 'Failed', itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
                    { offset: 0, color: isDark ? '#f85149' : '#cf222e' }, { offset: 1, color: isDark ? '#da3633' : '#a40e26' }
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
    const isDark = document.documentElement.classList.contains('dark-theme');

    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
            borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 8,
            padding: [10, 14],
            textStyle: { fontSize: 11, color: isDark ? '#cbd5e1' : '#475569' },
            formatter: function(params) {
                const idx = params[0].dataIndex;
                return timeLabels[idx] + '<br/>Error Rate: <strong>' + params[0].value.toFixed(3) + '%</strong><br/>' +
                       'Failed: ' + (fails[idx] || 0) + ' / Total: ' + (totals[idx] || 0);
            }
        },
        grid: { left: 50, right: 16, top: 20, bottom: 44 },
        dataZoom: [{ type: 'slider', height: 14, bottom: 2, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: isDark ? 'rgba(239,68,68,0.10)' : 'rgba(220,38,38,0.10)', handleStyle: { color: tc.dangerColor }, textStyle: { color: tc.textColor, fontSize: 9 }, showDetail: false }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 9, interval: Math.floor(timeLabels.length / 8) } },
        yAxis: { type: 'value', name: '%', min: 0, max: Math.max(maxErrRate * 1.5, globalErrRate * 1.5, 1), axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(3) + '%' } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
        markLine: {
            silent: true,
            data: [{ yAxis: globalErrRate, label: { formatter: 'Total: ' + globalErrRate.toFixed(3) + '%', color: tc.dangerColor, fontSize: 9 }, lineStyle: { color: tc.dangerColor, type: 'dashed', width: 1 } }],
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
                { offset: 0, color: isDark ? 'rgba(239,68,68,0.18)' : 'rgba(220,38,38,0.18)' }, { offset: 1, color: isDark ? 'rgba(239,68,68,0.01)' : 'rgba(220,38,38,0.01)' }
            ])},
            symbol: 'none'
        }]
    });
}

function renderLatencyChart() {
    const m = reportData.metrics || {};
    const chart = echarts.init(document.getElementById('latencyChart'));

    const isDark = document.documentElement.classList.contains('dark-theme');

    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            confine: true,
            backgroundColor: isDark ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
            borderColor: isDark ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: isDark ? '#e6edf3' : '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.10);',
            formatter: function(params) {
                const labels = { Avg: '平均延迟', P50: '中位数 (50%)', P90: 'P90 延迟', P95: 'P95 延迟', P99: 'P99 尾部延迟' };
                return '<div style="font-weight:600;margin-bottom:4px">' + (labels[params[0].name] || params[0].name) + '</div>' +
                       '<div style="font-size:13px"><strong>' + params[0].value.toFixed(3) + '</strong> ms</div>';
            }
        },
        grid: { left: 44, right: 20, top: 24, bottom: 32 },
        xAxis: {
            type: 'category',
            data: ['Avg', 'P50', 'P90', 'P95', 'P99'],
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: tc.textColor, fontSize: 12, fontWeight: 600, margin: 12 }
        },
        yAxis: {
            type: 'value',
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: tc.lineColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } },
            splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }
        },
        series: [{
            type: 'bar',
            barWidth: '40%',
            data: [
                { value: parseFloat(m.avg_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: tc.latencyColors[0] }, { offset: 1, color: isDark ? '#2ea043' : '#15803d' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p50_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: tc.latencyColors[0] }, { offset: 1, color: isDark ? '#2ea043' : '#15803d' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p90_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: tc.latencyColors[1] }, { offset: 1, color: isDark ? '#d29922' : '#9a6700' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p95_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: tc.latencyColors[2] }, { offset: 1, color: isDark ? '#db6d28' : '#8a3a00' }
                ]), borderRadius: [6, 6, 2, 2] }},
                { value: parseFloat(m.p99_latency_s || 0) * 1000, itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: tc.latencyColors[3] }, { offset: 1, color: isDark ? '#da3633' : '#a40e26' }
                ]), borderRadius: [6, 6, 2, 2] }}
            ],
            label: {
                show: true, position: 'top',
                formatter: function(p) { return p.value.toFixed(3); },
                color: tc.textColor, fontSize: 11, fontWeight: 600, offset: [0, -4]
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
    const isDark = document.documentElement.classList.contains('dark-theme');
    
    const chart = echarts.init(document.getElementById('qpsChart'));
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
            borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 12,
            padding: [12, 16],
            textStyle: { fontSize: 11, color: isDark ? '#cbd5e1' : '#475569' },
            extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
            formatter: function(params) {
                const idx = params[0].dataIndex;
                return '<div style="font-size:11.5px;color:' + (isDark ? '#e6edf3' : '#1e293b') + ';margin-bottom:6px;font-weight:600">' + timeLabels[idx] + '</div>' +
                       'QPS: <strong>' + params[0].value.toFixed(3) + '</strong>';
            }
        },
        grid: { left: 50, right: 20, top: 20, bottom: 50 },
        dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(45,212,191, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: { type: 'value', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(3) } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
        series: [{
            name: 'QPS',
            type: 'line',
            smooth: isSmooth,
            step: isSmooth ? false : 'middle',
            data: qpsData,
            lineStyle: { width: 2, color: tc.colors[0] },
            itemStyle: { color: tc.colors[0] },
            areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: 'rgba(45,212,191, 0.15)' }, { offset: 1, color: 'rgba(45,212,191, 0.01)' }
            ])},
            symbol: 'none'
        }]
    });
}

function renderLatencyTrend() {
    const m = reportData.metrics || {};
    const timestamps = m.timestamps || [];
    const isSmooth = chartTypes.latTrend === 'smooth';
    const isDark = document.documentElement.classList.contains('dark-theme');
    
    if (!timestamps.length) return;
    
    const timeLabels = timestamps.map(ts => ts.substring(11, 19));
    const chart = echarts.init(document.getElementById('latencyTrendChart'));
    
    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
            borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 12,
            padding: [12, 16],
            textStyle: { fontSize: 11, color: isDark ? '#cbd5e1' : '#475569' },
            extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
            formatter: function(params) {
                let h = '<div style="font-size:11.5px;color:' + (isDark ? '#e6edf3' : '#1e293b') + ';margin-bottom:6px;font-weight:600">' + timeLabels[params[0].dataIndex] + '</div>';
                params.forEach(function(item) {
                    h += item.marker + ' ' + item.seriesName + ': <strong>' + item.value.toFixed(3) + '</strong>ms<br/>';
                });
                return h;
            }
        },
        grid: { top: 30, right: 20, bottom: 50, left: 50 },
        dataZoom: [{ type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(45,212,191, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true }],
        xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
        yAxis: [
            { type: 'value', position: 'left', axisLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } } },
            { type: 'value', position: 'right', axisLine: { show: false }, splitLine: { show: false }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: function(v) { return v.toFixed(0) } } }
        ],
        series: [
            { name: 'P50', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p50 || [], yAxisIndex: 0, lineStyle: { width: 2, color: tc.latencyColors[0] }, itemStyle: { color: tc.latencyColors[0] }, symbol: 'none' },
            { name: 'P90', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p90 || [], yAxisIndex: 0, lineStyle: { width: 2, color: tc.latencyColors[1] }, itemStyle: { color: tc.latencyColors[1] }, symbol: 'none' },
            { name: 'P95', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p95 || [], yAxisIndex: 0, lineStyle: { width: 2, color: tc.latencyColors[2] }, itemStyle: { color: tc.latencyColors[2] }, symbol: 'none' },
            { name: 'P99', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle', data: m.ts_p99 || [], yAxisIndex: 0, lineStyle: { width: 2, color: tc.latencyColors[3] }, itemStyle: { color: tc.latencyColors[3] }, symbol: 'none' }
        ]
    });
}

function renderNodeCharts() {
    const nodes = reportData.node_metrics || [];
    const isDark = document.documentElement.classList.contains('dark-theme');
    nodes.forEach(function(node, idx) {
        const timestamps = node.timestamps || [];
        const qpsData = node.ts_qps || [];
        const timeLabels = timestamps.map(ts => ts.substring(11, 19));
        const isSmooth = chartTypes['node-' + idx] === 'smooth';

        if (!timestamps.length) return;

        // ===== QPS 面板 =====
        const qpsEl = document.getElementById('nodeQpsChart' + idx);
        let qChart = null;
        if (qpsEl) {
            qChart = echarts.init(qpsEl);
            qChart.setOption({
                backgroundColor: tc.bg,
                tooltip: {
                    trigger: 'axis', confine: true,
                    backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
                    borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
                    borderWidth: 1, borderRadius: 12, padding: [12, 16],
                    textStyle: { fontSize: 11, color: isDark ? '#cbd5e1' : '#475569' },
                    extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
                    formatter: function(params) {
                        return '<div style="font-size:11.5px;color:' + (isDark ? '#e6edf3' : '#1e293b') + ';margin-bottom:6px;font-weight:600">' + timeLabels[params[0].dataIndex] + '</div>' +
                               params[0].marker + ' QPS: <strong>' + params[0].value.toFixed(3) + '</strong> req/s';
                    }
                },
                grid: { left: 50, right: 20, top: 16, bottom: 28 },
                xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
                yAxis: { type: 'value', name: 'req/s', nameTextStyle: { color: tc.textColor, fontSize: 10 }, axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
                series: [{
                    name: 'QPS', type: 'line', smooth: isSmooth, step: isSmooth ? false : 'middle',
                    data: qpsData, lineStyle: { width: 2, color: tc.colors[0] }, itemStyle: { color: tc.colors[0] }, symbol: 'none',
                    areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                        { offset: 0, color: 'rgba(45,212,191, 0.35)' }, { offset: 1, color: 'rgba(45,212,191, 0.03)' }
                    ]) }
                }]
            });
        }

        // ===== 延迟百分位带面板 =====
        const latEl = document.getElementById('nodeLatencyChart' + idx);
        let lChart = null;
        if (latEl) {
            const p50 = node.ts_p50 || [];
            const p90 = node.ts_p90 || [];
            const p95 = node.ts_p95 || [];
            const p99 = node.ts_p99 || [];
            const lc = tc.latencyColors;
            lChart = echarts.init(latEl);
            lChart.setOption({
                backgroundColor: tc.bg,
                tooltip: {
                    trigger: 'axis', confine: true,
                    backgroundColor: isDark ? 'rgba(30,41,59,0.95)' : 'rgba(255,255,255,0.96)',
                    borderColor: isDark ? 'rgba(71,85,105,0.3)' : 'rgba(148,163,184,0.2)',
                    borderWidth: 1, borderRadius: 12, padding: [12, 16],
                    textStyle: { fontSize: 11, color: isDark ? '#cbd5e1' : '#475569' },
                    extraCssText: 'box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.08); backdrop-filter: blur(8px);',
                    formatter: function(params) {
                        const i = params[0].dataIndex;
                        let h = '<div style="font-size:11.5px;color:' + (isDark ? '#e6edf3' : '#1e293b') + ';margin-bottom:6px;font-weight:600">' + timeLabels[i] + '</div>';
                        var rows = [
                            { name: 'P50', val: p50[i], color: lc[0] },
                            { name: 'P90', val: p90[i], color: lc[1] },
                            { name: 'P95', val: p95[i], color: lc[2] },
                            { name: 'P99', val: p99[i], color: lc[3] }
                        ];
                        rows.forEach(function(r) {
                            h += '<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:' + r.color + ';margin-right:6px"></span>' + r.name + ': <strong>' + Number(r.val).toFixed(3) + ' ms</strong><br/>';
                        });
                        return h;
                    }
                },
                legend: { data: ['P50','P99'], textStyle: { color: tc.textColor, fontSize: 10 }, top: 0, itemWidth: 14, itemHeight: 8 },
                grid: { left: 50, right: 20, top: 30, bottom: 50 },
                dataZoom: [
                    { type: 'slider', height: 18, bottom: 4, borderColor: 'transparent', backgroundColor: tc.lineColor, fillerColor: 'rgba(45,212,191, 0.15)', handleStyle: { color: tc.colors[0] }, textStyle: { color: tc.textColor, fontSize: 10 }, brushSelect: true },
                    { type: 'inside' }
                ],
                xAxis: { type: 'category', data: timeLabels, axisLine: { lineStyle: { color: tc.lineColor } }, axisLabel: { color: tc.textColor, fontSize: 10 } },
                yAxis: { type: 'value', name: 'ms', nameTextStyle: { color: tc.textColor, fontSize: 10 }, axisLine: { show: false }, splitLine: { lineStyle: { color: tc.lineColor, type: 'dashed' } }, axisLabel: { color: tc.textColor, fontSize: 10, formatter: '{value}ms' } },
                series: [
                    // 百分位带（堆叠）：P99>P95>P90>P50 从上到下，色带宽度=差值
                    // 深色模式：alpha 递增 (0.22→0.30→0.38) 避免层次模糊；浅色保持不变
                    { name: 'P95-P99', type: 'line', smooth: isSmooth, data: p99, lineStyle: { opacity: 0 }, itemStyle: { color: lc[3] }, stack: 'lat-band', symbol: 'none', areaStyle: { color: isDark ? 'rgba(248,81,73,0.38)' : 'rgba(248,81,73,0.16)' }, z: 2 },
                    { name: 'P90-P95', type: 'line', smooth: isSmooth, data: p95, lineStyle: { opacity: 0 }, itemStyle: { color: lc[2] }, stack: 'lat-band', symbol: 'none', areaStyle: { color: isDark ? 'rgba(240,136,62,0.30)' : 'rgba(240,136,62,0.16)' }, z: 2 },
                    { name: 'P50-P90', type: 'line', smooth: isSmooth, data: p90, lineStyle: { opacity: 0 }, itemStyle: { color: lc[1] }, stack: 'lat-band', symbol: 'none', areaStyle: { color: isDark ? 'rgba(227,179,65,0.22)' : 'rgba(227,179,65,0.14)' }, z: 2 },
                    { name: 'P50', type: 'line', smooth: isSmooth, data: p50, lineStyle: { width: 1.5, color: lc[0] }, itemStyle: { color: lc[0] }, stack: 'lat-band', symbol: 'none', areaStyle: { color: isDark ? 'rgba(63,185,80,0.10)' : 'rgba(63,185,80,0.04)' }, z: 3 },
                    // P99 顶层边界线（独立非堆叠）
                    { name: 'P99', type: 'line', smooth: isSmooth, data: p99, lineStyle: { width: 2.5, color: lc[3] }, itemStyle: { color: lc[3] }, symbol: 'none', z: 4 }
                ]
            });
        }

        // 同步 dataZoom：延迟面板是主控（有 slider），QPS 单向跟随避免循环
        if (qChart && lChart) {
            lChart.on('dataZoom', function(params) {
                var start = (params.batch && params.batch[0]) ? params.batch[0].start : params.start;
                var end = (params.batch && params.batch[0]) ? params.batch[0].end : params.end;
                if (typeof start === 'number' && typeof end === 'number') {
                    qChart.dispatchAction({ type: 'dataZoom', start: start, end: end });
                }
            });
        }
    });
}

function renderErrorBreakdownChart() {
    const errors = reportData.error_breakdown || [];
    if (!errors.length) return;

    const chart = echarts.init(document.getElementById('errorBreakdownChart'));
    const data = errors.map(function(e) {
        return { value: e.count, name: e.code || e.status || e.error_type || 'Unknown' };
    });

    const isDark = document.documentElement.classList.contains('dark-theme');

    chart.setOption({
        backgroundColor: tc.bg,
        tooltip: {
            trigger: 'item',
            confine: true,
            backgroundColor: isDark ? 'rgba(22,27,34,0.96)' : 'rgba(255,255,255,0.97)',
            borderColor: isDark ? 'rgba(48,54,61,0.8)' : 'rgba(208,215,222,0.5)',
            borderWidth: 1,
            borderRadius: 10,
            padding: [12, 16],
            textStyle: { fontSize: 12, color: isDark ? '#e6edf3' : '#24292f' },
            extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.12);',
            formatter: function(p) {
                return '<div style="font-weight:600;margin-bottom:4px">' + p.name + '</div>' +
                       '<div style="font-size:13px"><strong>' + p.value.toLocaleString() + '</strong> occurrences (' + p.percent.toFixed(3) + '%)</div>';
            }
        },
        legend: {
            orient: 'vertical', right: 20, top: 'center',
            textStyle: { color: tc.textColor }
        },
        series: [{
            type: 'pie',
            radius: ['45%', '70%'],
            center: ['40%', '50%'],
            data: data,
            label: { show: true, formatter: '{b}: {c}', color: tc.textColor },
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
            echarts.getInstanceByDom(document.getElementById('nodeQpsChart' + idx))?.resize();
            echarts.getInstanceByDom(document.getElementById('nodeLatencyChart' + idx))?.resize();
        });
    }
});

window.addEventListener('beforeprint', function() {
    var chartIds = ['overviewChart', 'errorRateChart', 'latencyChart', 'qpsChart', 'latencyTrendChart',
                     'sysGoroutineChart', 'sysHeapChart', 'sysCpuChart', 'sysTaskWaitChart'];
    if (reportData.node_metrics) {
        reportData.node_metrics.forEach(function(_, idx) { chartIds.push('nodeQpsChart' + idx); chartIds.push('nodeLatencyChart' + idx); });
    }
    chartIds.forEach(function(id) {
        var chart = echarts.getInstanceByDom(document.getElementById(id));
        if (chart) {
            var url = chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#fff' });
            var img = document.createElement('img');
            img.src = url;
            img.style.width = '100%';
            img.style.height = 'auto';
            var container = document.getElementById(id);
            if (container) {
                container.innerHTML = '';
                container.appendChild(img);
            }
        }
    });
});

function applyDarkMode(isDark) {
    tc.bg = isDark ? '#0d1117' : '#ffffff';
    tc.textColor = isDark ? '#c9d1d9' : '#656d76';
    tc.lineColor = isDark ? '#30363d' : '#dde2e8';
    // 主色系：蓝绿/绿/黄/橙/红/蓝/灰
    tc.colors = isDark
        ? ['#2dd4bf', '#3fb950', '#e3b341', '#f0883e', '#f85149', '#58a6ff', '#94a3b8']
        : ['#0d9488', '#1a7f37', '#bf8700', '#bc4c00', '#cf222e', '#0969da', '#64748b'];
    // 延迟曲线语义色：P50 绿(好) → P90 黄(注意) → P95 橙(警告) → P99 红(危险)
    tc.latencyColors = isDark
        ? ['#3fb950', '#e3b341', '#f0883e', '#f85149']
        : ['#1a7f37', '#bf8700', '#bc4c00', '#cf222e'];
    tc.dangerColor = isDark ? '#f85149' : '#cf222e';

    var chartIds = ['overviewChart', 'errorRateChart', 'latencyChart', 'qpsChart', 'latencyTrendChart',
                     'sysGoroutineChart', 'sysHeapChart', 'sysCpuChart', 'sysTaskWaitChart', 'sysQueueChart'];
    if (reportData.node_metrics) {
        reportData.node_metrics.forEach(function(_, idx) { chartIds.push('nodeQpsChart' + idx); chartIds.push('nodeLatencyChart' + idx); });
    }

    chartIds.forEach(function(id) {
        var chart = echarts.getInstanceByDom(document.getElementById(id));
        if (chart) {
            chart.dispose();
        }
    });

    renderOverviewChart();
    renderErrorRateChart();
    renderLatencyChart();
    renderQPSTrend();
    renderLatencyTrend();
    renderNodeCharts();
    {{if .SystemMetrics}}
    renderSysGoroutineChart();
    renderSysHeapChart();
    renderSysCpuChart();
    renderSysTaskWaitChart();
    renderSysQueueChart();
    {{end}}
}

let isDarkMode = false;

function toggleTheme() {
    isDarkMode = !isDarkMode;
    applyTheme();
}

function updateThemeIcon() {
    var sunIcon = document.getElementById('themeIconSun');
    var moonIcon = document.getElementById('themeIconMoon');
    var themeText = document.getElementById('themeText');
    if (sunIcon && moonIcon) {
        sunIcon.style.display = isDarkMode ? 'block' : 'none';
        moonIcon.style.display = isDarkMode ? 'none' : 'block';
    }
    if (themeText) {
        themeText.textContent = isDarkMode ? '浅色' : '深色';
    }
}

function applyTheme() {
    applyDarkMode(isDarkMode);
    document.documentElement.classList.toggle('dark-theme', isDarkMode);
    updateThemeIcon();
    // Re-render error breakdown chart so it picks up the new tc.bg
    if (reportData.error_breakdown && reportData.error_breakdown.length) {
        var ebChart = echarts.getInstanceByDom(document.getElementById('errorBreakdownChart'));
        if (ebChart) { ebChart.dispose(); }
        renderErrorBreakdownChart();
    }
}

applyTheme();
</script>
</body>
</html>`))

type EnhancedReportContext struct {
	Metrics        *EnhancedMetrics         `json:"metrics"`
	NodeMetrics    []EnhancedNodeMetric     `json:"node_metrics"`
	ErrorBreakdown []map[string]interface{} `json:"error_breakdown"`
	Metadata       *EnhancedMetadata        `json:"metadata"`
	SystemMetrics  *EnhancedSystemMetrics   `json:"system_metrics,omitempty"`
	JSONData       template.JS              `json:"-"`
	EChartsJS      template.JS              `json:"-"`
}

// EnhancedSystemMetrics holds system performance data for the exported HTML report.
type EnhancedSystemMetrics struct {
	Summary    EnhancedSystemMetricsSummary  `json:"summary"`
	TimeSeries []EnhancedSystemMetricsSample `json:"time_series,omitempty"`
}

// EnhancedSystemMetricsSummary holds aggregated system metrics for the exported HTML report.
type EnhancedSystemMetricsSummary struct {
	SampleCount      int     `json:"sample_count"`
	GoroutineMax     int64   `json:"goroutine_max"`
	GoroutineAvg     float64 `json:"goroutine_avg"`
	HeapAllocMaxMB   float64 `json:"heap_alloc_max_mb"`
	HeapAllocAvgMB   float64 `json:"heap_alloc_avg_mb"`
	CPUMax           float64 `json:"cpu_max"`
	CPUAvg           float64 `json:"cpu_avg"`
	GCPauseTotalMs   float64 `json:"gc_pause_total_ms"`
	GCCount          uint32  `json:"gc_count"`
	TaskWaitAvgMs    float64 `json:"task_wait_avg_ms"`
	TaskWaitP99MaxMs float64 `json:"task_wait_p99_max_ms"`
	PendingQueueMax  int     `json:"pending_queue_max"`
	PendingQueueAvg  float64 `json:"pending_queue_avg"`
	ActiveWorkersMax int     `json:"active_workers_max"`
	ActiveWorkersAvg float64 `json:"active_workers_avg"`
}

// EnhancedSystemMetricsSample holds a single system metrics sample for the exported HTML report.
type EnhancedSystemMetricsSample struct {
	Timestamp       string  `json:"timestamp"`
	GoroutineCount  int64   `json:"goroutine_count"`
	HeapAllocMB     float64 `json:"heap_alloc_mb"`
	HeapSysMB       float64 `json:"heap_sys_mb"`
	CPUUsagePercent float64 `json:"cpu_percent"`
	ActiveWorkers   int     `json:"active_workers"`
	PendingQueueLen int     `json:"pending_queue_len"`
	TaskWaitP50Ms   float64 `json:"task_wait_p50_ms"`
	TaskWaitP95Ms   float64 `json:"task_wait_p95_ms"`
	TaskWaitP99Ms   float64 `json:"task_wait_p99_ms"`
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
	ctx.EChartsJS = template.JS(echartsMinJS)

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

	// Populate system metrics if available.
	if detail.SystemMetrics != nil {
		sm := detail.SystemMetrics
		enhancedSM := &EnhancedSystemMetrics{
			Summary: EnhancedSystemMetricsSummary{
				SampleCount:      sm.Summary.SampleCount,
				GoroutineMax:     sm.Summary.GoroutineMax,
				GoroutineAvg:     sm.Summary.GoroutineAvg,
				HeapAllocMaxMB:   sm.Summary.HeapAllocMaxMB,
				HeapAllocAvgMB:   sm.Summary.HeapAllocAvgMB,
				CPUMax:           sm.Summary.CPUMax,
				CPUAvg:           sm.Summary.CPUAvg,
				GCPauseTotalMs:   sm.Summary.GCPauseTotalMs,
				GCCount:          sm.Summary.GCCount,
				TaskWaitAvgMs:    sm.Summary.TaskWaitAvgMs,
				TaskWaitP99MaxMs: sm.Summary.TaskWaitP99MaxMs,
				PendingQueueMax:  sm.Summary.PendingQueueMax,
				PendingQueueAvg:  sm.Summary.PendingQueueAvg,
				ActiveWorkersMax: sm.Summary.ActiveWorkersMax,
				ActiveWorkersAvg: sm.Summary.ActiveWorkersAvg,
			},
		}
		for _, s := range sm.TimeSeries {
			enhancedSM.TimeSeries = append(enhancedSM.TimeSeries, EnhancedSystemMetricsSample{
				Timestamp:       s.Timestamp.Format("2006-01-02 15:04:05"),
				GoroutineCount:  s.GoroutineCount,
				HeapAllocMB:     s.HeapAllocMB,
				HeapSysMB:       s.HeapSysMB,
				CPUUsagePercent: s.CPUUsagePercent,
				ActiveWorkers:   s.ActiveWorkers,
				PendingQueueLen: s.PendingQueueLen,
				TaskWaitP50Ms:   s.TaskWaitP50Ms,
				TaskWaitP95Ms:   s.TaskWaitP95Ms,
				TaskWaitP99Ms:   s.TaskWaitP99Ms,
			})
		}
		ctx.SystemMetrics = enhancedSM
	}

	return ctx
}
