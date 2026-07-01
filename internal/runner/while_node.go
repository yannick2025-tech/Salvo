package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// whileConfig holds the parsed configuration for a while node.
type whileConfig struct {
	ExitConditions      []exitCondition `json:"exit_conditions"`
	IntervalSeconds     int             `json:"interval_seconds"`
	MaxIterations       int             `json:"max_iterations"`
	MaxDurationMinutes  int             `json:"max_duration_minutes"`
	Steps               []stepConfig    `json:"steps"`
	FailAfterConsecutive int            `json:"fail_after_consecutive"`
	FailMessage         string          `json:"fail_message"`
}

// exitCondition defines a condition to check for loop exit.
type exitCondition struct {
	Variable string `json:"variable"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// stepConfig defines a child step within a while node.
type stepConfig struct {
	Name         string                  `json:"name"`
	Condition    *stepConditionConfig    `json:"condition,omitempty"`
	Request      *stepRequestConfig      `json:"request,omitempty"`
	Extract      []extractEntry          `json:"extract,omitempty"`
	ThinkTime    *thinkTimeConfig        `json:"think_time,omitempty"`
	TimedTrigger *timedTriggerConfig     `json:"timed_trigger,omitempty"`
	Retry        *retryConfig            `json:"retry,omitempty"`
	FailAfterConsecutive int             `json:"fail_after_consecutive,omitempty"`
	FailMessage  string                  `json:"fail_message,omitempty"`
}

// stepConditionConfig defines a condition for step execution.
type stepConditionConfig struct {
	Variable string `json:"variable"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// stepRequestConfig defines an HTTP request within a step.
type stepRequestConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	Form    *stepFormConfig   `json:"form,omitempty"`
}

// stepFormConfig defines a multipart/form-data body for a step request.
type stepFormConfig struct {
	Fields map[string]string `json:"fields,omitempty"`
	Files  map[string]string `json:"files,omitempty"`
}

// extractEntry maps a JSON path to a variable name.
type extractEntry struct {
	// JSON format: {"variableName": "$.path.to.value"}
	Variable string `json:"variable"`
	// For simple map format: map[string]string with one entry
	// We'll parse this flexibly.
	Path string `json:"path"`
}

// thinkTimeConfig defines a random delay range in milliseconds.
type thinkTimeConfig struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// timedTriggerConfig defines an async timed trigger.
type timedTriggerConfig struct {
	AfterSeconds string `json:"after_seconds"` // supports ${var} references
	Once         bool   `json:"once"`
}

// retryConfig defines retry behavior.
type retryConfig struct {
	MaxAttempts int `json:"max_attempts"`
	On429       string `json:"on_429,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for extractEntry.
// Supports both formats:
//   - {"variable": "chargingStatus", "path": "$.result.status"}
//   - {"chargingStatus": "$.result.status"} (single-key map)
func (e *extractEntry) UnmarshalJSON(data []byte) error {
	// Try structured format first.
	var structured struct {
		Variable string `json:"variable"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(data, &structured); err == nil && (structured.Variable != "" || structured.Path != "") {
		e.Variable = structured.Variable
		e.Path = structured.Path
		return nil
	}

	// Try single-key map format.
	var kv map[string]string
	if err := json.Unmarshal(data, &kv); err == nil {
		for k, v := range kv {
			e.Variable = k
			e.Path = v
			return nil
		}
	}

	return fmt.Errorf("invalid extract entry: %s", string(data))
}

func (n *sceneNode) executeWhile(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
	var cfg whileConfig
	if err := json.Unmarshal([]byte(n.config), &cfg); err != nil {
		nodeLog.Error("failed to parse while node config", logger.F("error", err))
		return nil, fmt.Errorf("parse while config: %w", err)
	}

	if len(cfg.Steps) == 0 {
		nodeLog.Warn("while node has no steps, skipping")
		return &dag.Output{Response: map[string]any{"node_id": n.id, "type": "while", "iterations": 0}}, nil
	}

	if len(cfg.ExitConditions) == 0 && cfg.MaxIterations <= 0 {
		return nil, fmt.Errorf("while node %s: must have exit_conditions or max_iterations (infinite loop protection)", n.id)
	}

	// Initialize loop variables from input.
	loopVars := make(map[string]any)
	if input != nil && input.Variables != nil {
		for k, v := range input.Variables {
			loopVars[k] = v
		}
	}

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 0 // unlimited
	}

	var maxDuration time.Duration
	if cfg.MaxDurationMinutes > 0 {
		maxDuration = time.Duration(cfg.MaxDurationMinutes) * time.Minute
	}

	consecutiveFailures := make(map[int]int) // step index → consecutive failure count
	triggeredSteps := make(map[int]bool)     // step index → timed trigger already fired
	var triggeredMu sync.Mutex
	var varsMu sync.RWMutex

	iteration := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			nodeLog.Warn("while loop cancelled", logger.F("iteration", iteration), logger.F("error", ctx.Err()))
			return nil, fmt.Errorf("while loop cancelled at iteration %d: %w", iteration, ctx.Err())
		default:
		}

		iteration++

		// Check max iterations.
		if maxIterations > 0 && iteration > maxIterations {
			nodeLog.Warn("while loop reached max iterations",
				logger.F("max_iterations", maxIterations),
				logger.F("iteration", iteration))
			return nil, fmt.Errorf("while loop reached max iterations (%d)", maxIterations)
		}

		// Check max duration.
		if maxDuration > 0 && time.Since(startTime) > maxDuration {
			nodeLog.Warn("while loop exceeded max duration",
				logger.F("max_duration", maxDuration),
				logger.F("elapsed", time.Since(startTime)))
			return nil, fmt.Errorf("while loop exceeded max duration (%v)", maxDuration)
		}

		nodeLog.Info("while loop iteration",
			logger.F("iteration", iteration),
			logger.F("max_iterations", maxIterations),
			logger.F("elapsed", time.Since(startTime).Round(time.Second).String()))

		// Execute steps sequentially within this iteration.
		for stepIdx, step := range cfg.Steps {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("while loop cancelled during step %q: %w", step.Name, ctx.Err())
			default:
			}

			nodeLog.Debug("executing while step", logger.F("step", stepIdx), logger.F("name", step.Name))

			// Step condition check: if condition is set and evaluates to false, skip the step.
			if step.Condition != nil {
				condExpr := fmt.Sprintf("${%s} %s \"%s\"", step.Condition.Variable, step.Condition.Operator, step.Condition.Value)
				varsMu.RLock()
				condMet := expr.EvaluateConditionExpr(condExpr, loopVars)
				varsMu.RUnlock()
				if !condMet {
					nodeLog.Debug("skipping step due to condition not met",
						logger.F("step", step.Name),
						logger.F("condition", condExpr))
					// Reset consecutive failure counter since this step was skipped, not failed.
					consecutiveFailures[stepIdx] = 0
					continue
				}
			}

			// Check if this step has a once-only timed trigger that already fired.
			if step.TimedTrigger != nil && step.TimedTrigger.Once {
				triggeredMu.Lock()
				if triggeredSteps[stepIdx] {
					triggeredMu.Unlock()
					consecutiveFailures[stepIdx] = 0
					continue
				}
				triggeredMu.Unlock()
			}

			// If step has a timed trigger with after_seconds, fire it asynchronously.
			if step.TimedTrigger != nil && step.TimedTrigger.AfterSeconds != "" {
				delayStr := resolveWithVariables(step.TimedTrigger.AfterSeconds, loopVars)
				delaySec, err := parseDurationSeconds(delayStr)
				if err != nil {
					nodeLog.Warn("invalid timed_trigger after_seconds", logger.F("raw", step.TimedTrigger.AfterSeconds), logger.F("error", err))
					continue
				}

				// Fire the trigger asynchronously.
				go func(idx int, s stepConfig) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(delaySec) * time.Second):
					}

					// Snapshot loopVars for read-only use (avoid concurrent map access).
					varsMu.RLock()
					localVars := make(map[string]any, len(loopVars))
					for k, v := range loopVars {
						localVars[k] = v
					}
					varsMu.RUnlock()

					// Execute the step's HTTP request.
					if s.Request != nil {
						req := buildStepHTTPRequest(s.Request, localVars, nodeLog)
						proto := httpprotocol.NewProtocol()
						resp, err := proto.Execute(ctx, req)
						if err != nil {
							nodeLog.Warn("timed trigger HTTP request failed",
								logger.F("step", s.Name),
								logger.F("url", s.Request.URL),
								logger.F("error", err))
							return
						}

						// Extract variables into local map, then merge back under lock.
						if httpResp, ok := resp.(*httpprotocol.HTTPResponse); ok && len(s.Extract) > 0 {
							localExtracted := make(map[string]any)
							extractVarsFromResponse(httpResp.Body, s.Extract, localExtracted)
							varsMu.Lock()
							for k, v := range localExtracted {
								loopVars[k] = v
							}
							varsMu.Unlock()
						}
					}

					triggeredMu.Lock()
					triggeredSteps[idx] = true
					triggeredMu.Unlock()

					nodeLog.Info("timed trigger executed",
						logger.F("step", s.Name),
						logger.F("after_seconds", delaySec))
				}(stepIdx, step)

				continue
			}

			// Execute HTTP request (if any).
			if step.Request != nil {
				maxAttempts := 1
				if step.Retry != nil && step.Retry.MaxAttempts > 0 {
					maxAttempts = step.Retry.MaxAttempts
				}

				var stepErr error
				for attempt := 0; attempt < maxAttempts; attempt++ {
					if attempt > 0 {
						nodeLog.Debug("retrying step",
							logger.F("step", step.Name),
							logger.F("attempt", attempt+1),
							logger.F("max", maxAttempts))
					}

					varsMu.Lock()
					stepErr = n.executeWhileStepHTTP(ctx, &step, loopVars, nodeLog)
					varsMu.Unlock()
					if stepErr == nil {
						break
					}

					// Check for 429 retry.
					if step.Retry != nil && step.Retry.On429 == "retry" && isHTTP429Error(stepErr) {
						continue
					}
					break
				}

				if stepErr != nil {
					_ = stepErr // used for consecutive failure tracking below
					consecutiveFailures[stepIdx]++
					nodeLog.Warn("while step failed",
						logger.F("step", step.Name),
						logger.F("consecutive_failures", consecutiveFailures[stepIdx]),
						logger.F("error", stepErr))

					// Check fail_after_consecutive.
					failThreshold := cfg.FailAfterConsecutive
					if step.FailAfterConsecutive > 0 {
						failThreshold = step.FailAfterConsecutive
					}
					if failThreshold > 0 && consecutiveFailures[stepIdx] >= failThreshold {
						msg := cfg.FailMessage
						if msg == "" {
							msg = step.FailMessage
						}
						if msg == "" {
							msg = fmt.Sprintf("step %q failed %d consecutive times", step.Name, failThreshold)
						}
						nodeLog.Error("while node failed due to consecutive failures",
							logger.F("step", step.Name),
							logger.F("failures", consecutiveFailures[stepIdx]),
							logger.F("message", msg))
						return nil, fmt.Errorf("while node: %s", msg)
					}
				} else {
					consecutiveFailures[stepIdx] = 0
				}
			}

			// Apply think_time (random delay).
			if step.ThinkTime != nil && step.ThinkTime.Min > 0 && step.ThinkTime.Max > 0 {
				delay := randomInt(step.ThinkTime.Min, step.ThinkTime.Max)
				nodeLog.Debug("think time delay", logger.F("delay_ms", delay))
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("while loop cancelled during think time: %w", ctx.Err())
				case <-time.After(time.Duration(delay) * time.Millisecond):
				}
			}
		}

		// Check exit conditions after each iteration.
		if len(cfg.ExitConditions) > 0 {
			allMet := true
			for _, ec := range cfg.ExitConditions {
				condExpr := fmt.Sprintf("${%s} %s \"%s\"", ec.Variable, ec.Operator, ec.Value)
				varsMu.RLock()
				ecMet := expr.EvaluateConditionExpr(condExpr, loopVars)
				varsMu.RUnlock()
				if !ecMet {
					allMet = false
					break
				}
			}
			if allMet {
				nodeLog.Info("while loop exit conditions met",
					logger.F("iteration", iteration),
					logger.F("elapsed", time.Since(startTime).Round(time.Second).String()))
				return &dag.Output{
					Response: map[string]any{
						"node_id":    n.id,
						"type":       "while",
						"iterations": iteration,
					},
				}, nil
			}
		}

		// Wait for interval before next iteration.
		if cfg.IntervalSeconds > 0 {
			if maxIterations == 0 || iteration < maxIterations {
				nodeLog.Debug("waiting interval before next iteration",
					logger.F("interval_seconds", cfg.IntervalSeconds))
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("while loop cancelled during interval: %w", ctx.Err())
				case <-time.After(time.Duration(cfg.IntervalSeconds) * time.Second):
				}
			}
		}
	}
}

// executeWhileStepHTTP executes an HTTP request step in the while loop.
func (n *sceneNode) executeWhileStepHTTP(ctx context.Context, step *stepConfig, loopVars map[string]any, nodeLog logger.Logger) error {
	req := buildStepHTTPRequest(step.Request, loopVars, nodeLog)
	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("step %q HTTP request: %w", step.Name, err)
	}

	httpResp, ok := resp.(*httpprotocol.HTTPResponse)
	if !ok {
		return fmt.Errorf("step %q: unexpected response type %T", step.Name, resp)
	}

	if !httpResp.IsSuccess() {
		return fmt.Errorf("step %q HTTP %d", step.Name, httpResp.StatusCode)
	}

	if n.stats != nil {
		n.stats.RecordLatency(httpResp.Latency, true)
	}
	if n.httpOnlyStats != nil {
		n.httpOnlyStats.RecordLatency(httpResp.Latency, true)
	}
	if n.nodeStats != nil {
		n.nodeStats.RecordLatency(httpResp.Latency, true)
	}

	nodeLog.Debug("step HTTP request completed",
		logger.F("step", step.Name),
		logger.F("status", httpResp.StatusCode),
		logger.F("latency_ms", httpResp.Latency.Milliseconds()))

	// Extract variables from response.
	if len(step.Extract) > 0 {
		extractVarsFromResponse(httpResp.Body, step.Extract, loopVars)
	}

	return nil
}

// buildStepHTTPRequest constructs an HTTPRequest from a step config.
func buildStepHTTPRequest(cfg *stepRequestConfig, vars map[string]any, nodeLog logger.Logger) *httpprotocol.HTTPRequest {
	url := resolveWithVariables(cfg.URL, vars)

	method := cfg.Method
	if method == "" {
		method = "GET"
	}

	req := &httpprotocol.HTTPRequest{
		Method:  httpprotocol.Method(method),
		URL:     url,
		Headers: make(map[string]string),
	}

	for k, v := range cfg.Headers {
		req.Headers[k] = resolveWithVariables(v, vars)
	}

	if cfg.Body != nil {
		switch b := cfg.Body.(type) {
		case string:
			req.Body = []byte(resolveWithVariables(b, vars))
		case map[string]any:
			bodyBytes, err := json.Marshal(b)
			if err == nil {
				bodyStr := resolveWithVariables(string(bodyBytes), vars)
				req.Body = []byte(bodyStr)
			}
		default:
			if bodyBytes, err := json.Marshal(b); err == nil {
				bodyStr := resolveWithVariables(string(bodyBytes), vars)
				req.Body = []byte(bodyStr)
			}
		}
	}

	// multipart/form-data (Form) takes precedence over Body when both set.
	if cfg.Form != nil {
		form := &httpprotocol.FormData{
			Fields: make(map[string]string, len(cfg.Form.Fields)),
			Files:  make(map[string]string, len(cfg.Form.Files)),
		}
		for k, v := range cfg.Form.Fields {
			form.Fields[k] = resolveWithVariables(v, vars)
		}
		for k, v := range cfg.Form.Files {
			form.Files[k] = resolveWithVariables(v, vars)
		}
		req.Form = form
	}

	return req
}

// extractVarsFromResponse extracts variables from an HTTP response body
// using the configured extract entries and stores them in the vars map.
func extractVarsFromResponse(body []byte, extracts []extractEntry, vars map[string]any) {
	if len(body) == 0 {
		return
	}

	var jsonData map[string]any
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return
	}

	for _, ext := range extracts {
		value := resolveJSONPath(jsonData, ext.Path)
		if value != nil {
			vars[ext.Variable] = value
		}
	}
}

// resolveJSONPath resolves a simple JSON path (e.g., "$.result.status") against data.
// Supports: $.field, $.field.subfield, $.field[index], $.field.subfield[index]
func resolveJSONPath(data map[string]any, path string) any {
	if !strings.HasPrefix(path, "$") {
		return nil
	}

	// Strip "$." or "$" prefix.
	parts := strings.Split(path, ".")
	if parts[0] == "$" {
		parts = parts[1:]
	}

	if len(parts) == 0 {
		return data
	}

	current := any(data)
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check for array index: fieldName[index]
		if idx := strings.Index(part, "["); idx >= 0 {
			fieldName := part[:idx]
			indexStr := part[idx+1 : len(part)-1] // strip [ and ]

			// Navigate to the field.
			if m, ok := current.(map[string]any); ok {
				current = m[fieldName]
			} else {
				return nil
			}

			// Navigate into the array.
			arr, ok := current.([]any)
			if !ok {
				return nil
			}
			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return nil
			}
			if index < 0 || index >= len(arr) {
				return nil
			}
			current = arr[index]
		} else {
			if m, ok := current.(map[string]any); ok {
				current = m[part]
			} else {
				return nil
			}
		}
	}

	return current
}

// parseDurationSeconds parses a duration string as seconds.
func parseDurationSeconds(s string) (float64, error) {
	var sec float64
	if _, err := fmt.Sscanf(s, "%f", &sec); err != nil {
		return 0, err
	}
	return sec, nil
}

// isHTTP429Error checks if an error is an HTTP 429 error.
func isHTTP429Error(err error) bool {
	return strings.Contains(err.Error(), "429")
}

// randomInt returns a random integer in [min, max].
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	// Simple deterministic approach using time for now.
	// In production, use crypto/rand or math/rand with proper seeding.
	r := time.Now().UnixNano() % int64(max-min+1)
	return min + int(r)
}