// Package runner - node_postprocess.go
// 统一节点后处理：extract 和 retry
package runner

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// nodeRetryConfig 定义节点级重试配置（支持指数退避）
type nodeRetryConfig struct {
	MaxAttempts    int     `json:"max_attempts"`
	InitialBackoff string  `json:"initial_backoff"` // e.g., "100ms", "1s"
	Multiplier     float64 `json:"multiplier"`      // e.g., 2.0
	MaxBackoff     string  `json:"max_backoff"`     // e.g., "30s"
	Jitter         bool    `json:"jitter"`          // 是否添加随机抖动
	OnStatus       []int   `json:"on_status"`       // 触发重试的 HTTP 状态码，默认 [429, 503]
}

// nodeExtractEntry 定义 extract 条目
type nodeExtractEntry struct {
	Variable string `json:"variable"`
	Path     string `json:"path"`
}

// parseRetryConfig 从节点 config JSON 中解析 retry 配置
func (n *sceneNode) parseRetryConfig() *nodeRetryConfig {
	if n.config == "" {
		return nil
	}

	var cfg struct {
		Retry *nodeRetryConfig `json:"retry"`
	}
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		return nil
	}

	if cfg.Retry == nil {
		return nil
	}

	// 设置默认值
	if cfg.Retry.MaxAttempts <= 0 {
		cfg.Retry.MaxAttempts = 1
	}
	if cfg.Retry.InitialBackoff == "" {
		cfg.Retry.InitialBackoff = "100ms"
	}
	if cfg.Retry.Multiplier <= 0 {
		cfg.Retry.Multiplier = 2.0
	}
	if cfg.Retry.MaxBackoff == "" {
		cfg.Retry.MaxBackoff = "30s"
	}
	if len(cfg.Retry.OnStatus) == 0 {
		cfg.Retry.OnStatus = []int{429, 503}
	}

	return cfg.Retry
}

// parseExtractConfig 从节点 config JSON 中解析 extract 配置
// 支持两种格式：
// 1. 对象格式: {"extract": {"token": "$.data.token", "order_id": "$.data.orderId"}}
// 2. 数组格式: {"extract": [{"variable": "token", "path": "$.data.token"}]}
func (n *sceneNode) parseExtractConfig() []nodeExtractEntry {
	if n.config == "" {
		return nil
	}

	// 先尝试对象格式
	var objCfg struct {
		Extract map[string]string `json:"extract"`
	}
	if err := json.Unmarshal([]byte(n.config), &objCfg); err == nil && len(objCfg.Extract) > 0 {
		entries := make([]nodeExtractEntry, 0, len(objCfg.Extract))
		for varName, path := range objCfg.Extract {
			entries = append(entries, nodeExtractEntry{
				Variable: varName,
				Path:     path,
			})
		}
		return entries
	}

	// 再尝试数组格式
	var arrCfg struct {
		Extract []nodeExtractEntry `json:"extract"`
	}
	if err := json.Unmarshal([]byte(n.config), &arrCfg); err == nil && len(arrCfg.Extract) > 0 {
		return arrCfg.Extract
	}

	return nil
}

// parseDuration 解析时间字符串，支持 "100ms", "1s", "500" (默认毫秒) 等格式
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// 尝试直接解析为数字（毫秒）
	if ms, err := time.ParseDuration(s); err == nil {
		return ms, nil
	}

	// 尝试解析为纯数字（视为毫秒）
	var ms int64
	if _, err := fmt.Sscanf(s, "%d", &ms); err == nil {
		return time.Duration(ms) * time.Millisecond, nil
	}

	return 0, fmt.Errorf("invalid duration format: %s", s)
}

// calculateBackoff 计算指数退避时间
// 公式: backoff = initial * (multiplier ^ attempt)，上限为 maxBackoff
// 如果启用 jitter，则在 ±50% 范围内随机调整
func calculateBackoff(cfg *nodeRetryConfig, attempt int) time.Duration {
	initial, err := parseDuration(cfg.InitialBackoff)
	if err != nil {
		initial = 100 * time.Millisecond
	}

	maxBackoff, err := parseDuration(cfg.MaxBackoff)
	if err != nil {
		maxBackoff = 30 * time.Second
	}

	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	// 计算指数退避
	backoff := float64(initial)
	for i := 0; i < attempt; i++ {
		backoff *= cfg.Multiplier
	}

	// 应用上限
	if time.Duration(backoff) > maxBackoff {
		backoff = float64(maxBackoff)
	}

	// 应用 jitter（±50%）
	if cfg.Jitter {
		jitterFactor := 0.5 + rand.Float64() // 0.5 ~ 1.5
		backoff *= jitterFactor
	}

	return time.Duration(backoff)
}

// shouldRetry 判断是否应该重试
func shouldRetry(cfg *nodeRetryConfig, err error) bool {
	if cfg == nil || err == nil {
		return false
	}

	// 检查是否是 HTTP 状态码错误
	errMsg := err.Error()
	for _, status := range cfg.OnStatus {
		if strings.Contains(errMsg, fmt.Sprintf("status %d", status)) ||
			strings.Contains(errMsg, fmt.Sprintf("HTTP %d", status)) ||
			strings.Contains(errMsg, fmt.Sprintf("code %d", status)) {
			return true
		}
	}

	return false
}

// applyExtract 从节点响应中提取变量并写入共享作用域
func (n *sceneNode) applyExtract(out *dag.Output, extracts []nodeExtractEntry, input *dag.Input, nodeLog logger.Logger) {
	if out == nil || len(extracts) == 0 || input == nil || input.Executor == nil {
		return
	}

	// 获取响应体
	var responseBody []byte
	switch v := out.Response.(type) {
	case []byte:
		responseBody = v
	case string:
		responseBody = []byte(v)
	case map[string]any:
		// 如果是 map，序列化为 JSON
		if data, err := json.Marshal(v); err == nil {
			responseBody = data
		}
	case *httpprotocol.HTTPResponse:
		responseBody = v.Body
	default:
		nodeLog.Debug("extract skipped: unsupported response type",
			logger.F("type", fmt.Sprintf("%T", out.Response)),
		)
		return
	}

	if len(responseBody) == 0 {
		return
	}

	// 解析 JSON
	var jsonData map[string]any
	if err := json.Unmarshal(responseBody, &jsonData); err != nil {
		nodeLog.Warn("extract skipped: response is not valid JSON, variables will keep their current values",
			logger.F("error", err),
			logger.F("body_preview", truncateResponseBody(responseBody, 100)),
			logger.F("extract_count", len(extracts)),
		)
		return
	}

	// 提取变量
	for _, ext := range extracts {
		value := resolveJSONPath(jsonData, ext.Path)
		if value != nil {
			input.Executor.SetVariable(ext.Variable, value)
			nodeLog.Info("extracted variable",
				logger.F("variable", ext.Variable),
				logger.F("path", ext.Path),
				logger.F("value", value),
			)
		} else {
			nodeLog.Debug("extract path not found",
				logger.F("variable", ext.Variable),
				logger.F("path", ext.Path),
			)
		}
	}
}

// truncateResponseBody truncates a response body for logging.
func truncateResponseBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}


