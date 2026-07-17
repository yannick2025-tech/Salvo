package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/yannick2025-tech/Salvo/internal/runner"
)

var reportTemplate = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>测试报告 - Salvo</title>
    <style>
        :root {
            --primary: #0d9488;
            --primary-dark: #0f766e;
            --success: #1a7f37;
            --warning: #bf8700;
            --danger: #cf222e;
            --bg-primary: #ffffff;
            --bg-secondary: #f9fafb;
            --bg-tertiary: #f3f4f6;
            --text-primary: #111827;
            --text-secondary: #6b7280;
            --border-color: #e5e7eb;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            line-height: 1.6;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }

        .header {
            text-align: center;
            margin-bottom: 3rem;
            padding: 2rem;
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            color: white;
            border-radius: 16px;
            box-shadow: 0 4px 20px rgba(99, 102, 241, 0.3);
        }

        .header h1 {
            font-size: 2.5rem;
            font-weight: 700;
            margin-bottom: 0.5rem;
        }

        .header p {
            opacity: 0.95;
            font-size: 1.1rem;
        }

        .section {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            margin-bottom: 2rem;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
        }

        .section-title {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--text-primary);
            margin-bottom: 1.5rem;
            padding-bottom: 1rem;
            border-bottom: 2px solid var(--border-color);
        }

        .icon {
            width: 32px;
            height: 32px;
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: bold;
            color: white;
        }

        .icon-blue { background: linear-gradient(135deg, #0969da, #0a6fbb); }
        .icon-green { background: linear-gradient(135deg, #1a7f37, #15803d); }
        .icon-orange { background: linear-gradient(135deg, #bc4c00, #973604); }
        .icon-red { background: linear-gradient(135deg, #cf222e, #a40e26); }
        .icon-purple { background: linear-gradient(135deg, #0d9488, #0f766e); }

        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1.5rem;
        }

        .metric-card {
            background: var(--bg-secondary);
            padding: 1.25rem;
            border-radius: 12px;
            border-left: 4px solid var(--primary);
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .metric-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
        }

        .metric-label {
            font-size: 0.875rem;
            color: var(--text-secondary);
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .metric-value {
            font-size: 2rem;
            font-weight: 800;
            color: var(--text-primary);
            margin-top: 0.25rem;
        }

        .node-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }

        .node-table th,
        .node-table td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }

        .node-table th {
            background: var(--bg-secondary);
            font-weight: 600;
            color: var(--text-secondary);
            text-transform: uppercase;
            font-size: 0.85rem;
            letter-spacing: 0.03em;
        }

        .node-table tr:hover {
            background: var(--bg-tertiary);
        }

        .status-badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.875rem;
            font-weight: 600;
        }

        .status-success { background: #d1fae5; color: #065f46; }
        .status-warning { background: #fef3c7; color: #92400e; }
        .status-danger { background: #fee2e2; color: #991b1b; }

        @media (max-width: 768px) {
            .container { padding: 1rem; }
            .header h1 { font-size: 1.75rem; }
            .metrics-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 测试报告</h1>
            <p>{{.Metadata.SceneName}} - {{.Metadata.StartedAt.Format "2006-01-02 15:04:05"}}</p>
        </div>

        <!-- Global Summary -->
        <div class="section">
            <div class="section-title">
                <span class="icon icon-blue">📈</span>
                总体指标
            </div>
            <div class="metrics-grid">
                <div class="metric-card">
                    <div class="metric-label">总请求数</div>
                    <div class="metric-value">{{printf "%.0f" .GlobalSummary.TotalRequests}}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">成功率</div>
                    <div class="metric-value">{{printf "%.1f%%" .GlobalSummary.SuccessRate}}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">平均延迟 (ms)</div>
                    <div class="metric-value">{{printf "%.1f" .GlobalSummary.AvgLatencyMs}}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">P95 延迟 (ms)</div>
                    <div class="metric-value">{{printf "%.1f" .GlobalSummary.P95LatencyMs}}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">P99 延迟 (ms)</div>
                    <div class="metric-value">{{printf "%.1f" .GlobalSummary.P99LatencyMs}}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">吞吐量 (req/s)</div>
                    <div class="metric-value">{{printf "%.1f" .GlobalSummary.Throughput}}</div>
                </div>
            </div>
        </div>

        {{if gt (len .NodeMetrics) 0}}
        <!-- Node Metrics -->
        <div class="section">
            <div class="section-title">
                <span class="icon icon-purple">🔗</span>
                各节点运行时指标
            </div>
            <table class="node-table">
                <thead>
                    <tr>
                        <th>节点 ID</th>
                        <th>节点名称</th>
                        <th>总请求</th>
                        <th>成功/失败</th>
                        <th>成功率</th>
                        <th>P50/P95/P99 (ms)</th>
                        <th>平均 QPS</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .NodeMetrics}}
                    <tr>
                        <td><code>{{.NodeID}}</code></td>
                        <td>{{.NodeName}}</td>
                        <td>{{printf "%.0f" .Summary.TotalRequests}}</td>
                        <td>{{printf "%.0f" .Summary.SuccessCount}} / {{printf "%.0f" .Summary.FailCount}}</td>
                        <td><span class="status-badge status-success">{{printf "%.1f%%" .Summary.SuccessRate}}</span></td>
                        <td>{{printf "%.1f" .Summary.P50LatencyMs}} / {{printf "%.1f" .Summary.P95LatencyMs}} / {{printf "%.1f" .Summary.P99LatencyMs}}</td>
                        <td>{{printf "%.2f" .Summary.AvgQPS}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}

        {{if gt (len .FailedNodes) 0}}
        <!-- Failed Nodes Detail -->
        <div class="section">
            <div class="section-title">
                <span class="icon icon-red">❌</span>
                失败节点详情
            </div>
            {{range $idx, $fn := .FailedNodes}}
            <div style="margin-bottom: 1.5rem; border: 1px solid var(--border-color); border-radius: 8px; overflow: hidden;">
                <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; font-weight: 600; display: flex; justify-content: space-between; align-items: center;">
                    <span>{{.NodeName}} <code style="font-size: 0.8rem; color: var(--text-secondary);">({{.NodeType}})</code></span>
                    <span style="font-size: 0.85rem; color: var(--text-secondary);">{{.Timestamp.Format "15:04:05"}}</span>
                </div>
                <div style="padding: 1rem;">
                    <div style="margin-bottom: 0.5rem;">
                        <strong>错误信息:</strong> <span style="color: var(--danger);">{{.ErrorMessage}}</span>
                    </div>
                    {{if .RequestURL}}
                    <details style="margin-top: 0.75rem;">
                        <summary style="cursor: pointer; font-weight: 600; color: var(--text-secondary);">请求详情</summary>
                        <div style="margin-top: 0.5rem; padding: 0.75rem; background: var(--bg-secondary); border-radius: 6px; font-size: 0.85rem;">
                            <div><strong>Method:</strong> {{.RequestMethod}}</div>
                            <div><strong>URL:</strong> <code>{{.RequestURL}}</code></div>
                            {{if .RequestHeaders}}
                            <div style="margin-top: 0.5rem;"><strong>Headers:</strong></div>
                            <pre style="margin: 0.25rem 0; padding: 0.5rem; background: var(--bg-tertiary); border-radius: 4px; overflow-x: auto; font-size: 0.8rem;">{{range $k, $v := .RequestHeaders}}{{$k}}: {{$v}}
{{end}}</pre>
                            {{end}}
                            {{if .RequestBody}}
                            <div style="margin-top: 0.5rem;"><strong>Body:</strong></div>
                            <pre style="margin: 0.25rem 0; padding: 0.5rem; background: var(--bg-tertiary); border-radius: 4px; overflow-x: auto; font-size: 0.8rem;">{{.RequestBody}}</pre>
                            {{end}}
                        </div>
                    </details>
                    {{end}}
                    {{if or .ResponseBody .ResponseStatus}}
                    <details style="margin-top: 0.5rem;">
                        <summary style="cursor: pointer; font-weight: 600; color: var(--text-secondary);">响应详情</summary>
                        <div style="margin-top: 0.5rem; padding: 0.75rem; background: var(--bg-secondary); border-radius: 6px; font-size: 0.85rem;">
                            <div><strong>Status:</strong> <span class="status-badge status-danger">{{.ResponseStatus}}</span></div>
                            {{if .ResponseHeaders}}
                            <div style="margin-top: 0.5rem;"><strong>Headers:</strong></div>
                            <pre style="margin: 0.25rem 0; padding: 0.5rem; background: var(--bg-tertiary); border-radius: 4px; overflow-x: auto; font-size: 0.8rem;">{{range $k, $v := .ResponseHeaders}}{{$k}}: {{range $i, $vv := $v}}{{if $i}}, {{end}}{{$vv}}{{end}}
{{end}}</pre>
                            {{end}}
                            {{if .ResponseBody}}
                            <div style="margin-top: 0.5rem;"><strong>Body:</strong></div>
                            <pre style="margin: 0.25rem 0; padding: 0.5rem; background: var(--bg-tertiary); border-radius: 4px; overflow-x: auto; font-size: 0.8rem;">{{.ResponseBody}}</pre>
                            {{end}}
                        </div>
                    </details>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>
        {{end}}

        <footer style="text-align: center; padding: 2rem; color: var(--text-secondary);">
            <p>Generated by Salvo v{{.Metadata.Version}} at {{.Metadata.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}</p>
        </footer>
    </div>
</body>
</html>`))

func generateHTMLReport(detailJSON string) string {
	var detail runner.ReportDetail
	if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
		return fmt.Sprintf("<html><body><h1>Error parsing report data: %s</h1></body></html>", err)
	}

	var buf strings.Builder
	if err := reportTemplate.Execute(&buf, detail); err != nil {
		return fmt.Sprintf("<html><body><h1>Error generating report: %s</h1></body></html>", err)
	}

	return buf.String()
}
