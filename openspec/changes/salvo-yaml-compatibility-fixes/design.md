## Context

当前 Salvo DAG runner 存在以下问题：

1. **extract 支持不一致**：while/parallel/loop 的 step 已支持 extract，但顶层 HTTP/Generator 节点不支持
2. **retry 能力缺失**：只有 while step 支持简单重试（无退避），顶层节点无 retry，且现有 retry 无间隔等待
3. **while 步骤类型受限**：stepConfig 只支持 HTTP 请求，不支持 generator 类型
4. **multipart 格式不兼容**：card.yaml 使用扁平格式，代码期望嵌套格式
5. **think_time 架构不符**：card.yaml 使用节点级 think_time，但 Salvo 架构应使用独立 delay 节点

现有基础设施：
- `extractVarsFromResponse` 函数已在 while_node.go 中实现
- DAG executor 已支持 async/sync 模式
- HTTP protocol 已支持 timeout
- sceneNode.Execute 是统一的后处理入口

## Goals / Non-Goals

**Goals:**
- 在 sceneNode.Execute 层实现统一的 extract 和 retry 后处理
- 支持指数退避重试策略（initial_backoff、multiplier、max_backoff、jitter）
- While 步骤支持 generator 节点类型
- multipart 格式向后兼容（同时支持扁平和嵌套格式）
- 将 card.yaml 的 think_time 替换为 delay 节点

**Non-Goals:**
- 不改变 while/parallel/loop 的 step 级 extract/retry 实现
- 不添加 HTTP 节点的 async 配置能力（已有 async mode，但不在本变更范围）
- 不重构 YAML 导入/导出逻辑（仅处理运行时解析）
- 不修改 DAG executor 的核心逻辑

## Decisions

### Decision 1: 统一后处理架构

**选择**：在 sceneNode.Execute 的 switch 之后添加统一后处理层

```go
func (n *sceneNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
    // 1. 执行节点逻辑（现有 switch）
    var out *dag.Output
    var err error
    switch n.nodeType {
    case model.NodeTypeHTTP:
        out, err = n.executeHTTP(ctx, input, nodeLog)
    // ... 其他类型
    }
    
    // 2. 统一后处理
    if out != nil {
        // 2a. extract 提取
        if extractCfg := n.parseExtractConfig(); len(extractCfg) > 0 {
            n.applyExtract(out, extractCfg, input)
        }
        
        // 2b. retry 包装（需要在执行前判断，见 Decision 2）
    }
    
    return out, err
}
```

**理由**：
- 复用现有的 extractVarsFromResponse 逻辑
- 所有节点类型自动获得 extract 能力
- 与 while/parallel/loop 的 step 级 extract 保持一致

**替代方案**：
- 在每个 executeXXX 方法中单独处理 → 代码重复，维护成本高
- 在 DAG executor 层处理 → 侵入核心逻辑，影响范围大

### Decision 2: Retry 实现策略

**选择**：在 sceneNode.Execute 层包装重试逻辑

```go
func (n *sceneNode) Execute(ctx context.Context, input *dag.Input) (*dag.Output, error) {
    retryCfg := n.parseRetryConfig()
    maxAttempts := 1
    if retryCfg != nil && retryCfg.MaxAttempts > 0 {
        maxAttempts = retryCfg.MaxAttempts
    }
    
    var out *dag.Output
    var err error
    
    for attempt := 0; attempt < maxAttempts; attempt++ {
        if attempt > 0 {
            // 计算退避时间
            backoff := n.calculateBackoff(retryCfg, attempt)
            time.Sleep(backoff)
        }
        
        // 执行节点逻辑
        out, err = n.executeNodeLogic(ctx, input, nodeLog)
        
        // 判断是否需要重试
        if err == nil || !n.shouldRetry(retryCfg, err) {
            break
        }
    }
    
    // 后处理（extract 等）
    if out != nil {
        n.applyPostProcessing(out, input)
    }
    
    return out, err
}
```

**退避策略配置**：
```go
type RetryConfig struct {
    MaxAttempts    int     `json:"max_attempts"`
    InitialBackoff string  `json:"initial_backoff"` // "100ms", "1s"
    Multiplier     float64 `json:"multiplier"`      // 2.0
    MaxBackoff     string  `json:"max_backoff"`     // "30s"
    Jitter         bool    `json:"jitter"`          // true
    OnStatus       []int   `json:"on_status"`       // [429, 503]
}
```

**退避计算公式**：
```
backoff = initial_backoff * (multiplier ^ attempt)
if backoff > max_backoff {
    backoff = max_backoff
}
if jitter {
    backoff = backoff * (0.5 + rand(0.5))  // ±50% 抖动
}
```

**理由**：
- 统一处理所有节点类型的重试
- 指数退避避免雪崩效应
- Jitter 防止多个节点同步重试

**替代方案**：
- 固定间隔重试 → 不适合高并发场景
- 在每个节点类型中实现 → 代码重复

### Decision 3: While Generator Step

**选择**：扩展 stepConfig 添加 Type 和 Config 字段

```go
type stepConfig struct {
    Name         string                  `json:"name"`
    Type         string                  `json:"type,omitempty"` // "http" (default) | "generator"
    Config       map[string]any          `json:"config,omitempty"`
    Condition    *stepConditionConfig    `json:"condition,omitempty"`
    Request      *stepRequestConfig      `json:"request,omitempty"` // 兼容旧格式
    Extract      []extractEntry          `json:"extract,omitempty"`
    ThinkTime    *thinkTimeConfig        `json:"think_time,omitempty"`
    TimedTrigger *timedTriggerConfig     `json:"timed_trigger,omitempty"`
    Retry        *retryConfig            `json:"retry,omitempty"`
}
```

**执行逻辑**：
```go
func (n *sceneNode) executeWhileStep(ctx context.Context, step *stepConfig, loopVars map[string]any, nodeLog logger.Logger) error {
    stepType := step.Type
    if stepType == "" {
        stepType = "http" // 默认
    }
    
    switch stepType {
    case "http":
        return n.executeWhileStepHTTP(ctx, step, loopVars, nodeLog)
    case "generator":
        return n.executeWhileStepGenerator(ctx, step, loopVars, nodeLog)
    default:
        return fmt.Errorf("unsupported step type: %s", stepType)
    }
}

func (n *sceneNode) executeWhileStepGenerator(ctx context.Context, step *stepConfig, loopVars map[string]any, nodeLog logger.Logger) error {
    // 复用 executeGenerator 的逻辑
    expr, _ := step.Config["expression"].(string)
    variable, _ := step.Config["variable"].(string)
    
    // 解析变量
    resolved := resolveWithVariables(expr, loopVars)
    
    // 执行表达式
    result, err := n.evaluateExpression(resolved)
    if err != nil {
        return err
    }
    
    // 写回变量
    loopVars[variable] = result
    
    // 提取字段
    if len(step.Extract) > 0 {
        // 将 result 转为 map，然后提取
        if resultMap, ok := result.(map[string]any); ok {
            extractVarsFromMap(resultMap, step.Extract, loopVars)
        }
    }
    
    return nil
}
```

**理由**：
- 保持向后兼容（Type 为空时默认 http）
- 复用 executeGenerator 的核心逻辑
- 支持 generator 的 extract 能力

### Decision 4: Multipart 格式兼容

**选择**：在 executeHTTP 中同时支持两种格式

```go
func (n *sceneNode) executeHTTP(ctx context.Context, input *dag.Input, nodeLog logger.Logger) (*dag.Output, error) {
    var cfg struct {
        Method    string         `json:"method"`
        URL       string         `json:"url"`
        Headers   map[string]string `json:"headers"`
        Body      string         `json:"body"`
        Timeout   any            `json:"timeout"`
        Form      *FormConfig    `json:"form,omitempty"`      // 嵌套格式
        Multipart map[string]any `json:"multipart,omitempty"` // 扁平格式
    }
    
    // 解析配置
    json.Unmarshal([]byte(n.config), &cfg)
    
    // 统一转为 FormConfig
    var form *FormConfig
    if cfg.Form != nil {
        form = cfg.Form
    } else if len(cfg.Multipart) > 0 {
        form = &FormConfig{
            Fields: make(map[string]string),
            Files:  make(map[string]string),
        }
        for k, v := range cfg.Multipart {
            if str, ok := v.(string); ok {
                // 简单判断：包含文件路径的字段放 Files
                if strings.Contains(str, "/") || strings.Contains(str, ".") {
                    form.Files[k] = str
                } else {
                    form.Fields[k] = str
                }
            }
        }
    }
    
    // 后续逻辑使用 form
    // ...
}
```

**理由**：
- 向后兼容 card.yaml 的扁平格式
- 新配置推荐使用嵌套格式（更清晰）
- 自动推断 fields/files 降低使用门槛

**替代方案**：
- 只支持一种格式 → 需要修改 card.yaml
- 要求用户显式指定 fields/files → 增加配置复杂度

### Decision 5: 节点级独立超时

**选择**：在 DAG executor 中为每个节点创建独立的 timeout context

```go
func (e *Executor) executeNode(ctx context.Context, node Node, input *Input) (*Output, error) {
    // 获取节点配置的超时时间
    nodeTimeout := node.Timeout()
    if nodeTimeout > 0 {
        // 创建节点级独立超时 context
        nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
        defer cancel()
        ctx = nodeCtx
    }
    // 节点超时不影响其他节点，只影响当前节点
    return node.Execute(ctx, input)
}
```

**理由**：
- 节点超时相互隔离，一个节点的超时不会导致整个场景失败
- 支持长时运行的节点（如 delay 53秒）而不受场景默认超时限制
- 与场景级超时形成两级保护：场景超时兜底，节点超时精细控制

**替代方案**：
- 仅使用场景级超时 → 无法精细控制单个节点的超时行为
- 完全移除超时限制 → 失去保护机制，可能导致资源泄漏

### Decision 6: 场景默认超时配置

**选择**：在 Scene 模型中添加 `default_timeout` 字段，支持通过 GUI 配置

```go
// Scene 模型
type Scene struct {
    // ... 其他字段
    DefaultTimeout int `json:"default_timeout"` // 秒，0 表示使用系统默认
}

// Runner 使用逻辑
func (r *Runner) runScene(scene *Scene) error {
    timeout := scene.DefaultTimeout
    if timeout == 0 {
        timeout = 600 // 系统默认 10 分钟
    }
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
    defer cancel()
    // ...
}
```

**理由**：
- 不同场景有不同的超时需求（充电场景可能需要 10+ 分钟）
- 通过 GUI 配置，无需修改代码或重启服务
- 0 值表示使用系统默认，保持向后兼容

### Decision 7: 前端全节点编辑支持变量引用

**选择**：将所有节点类型的数值字段从 `type="number"` 改为 `type="text"`，支持变量引用

**修改的字段**：
- delay 节点：`ms` 字段
- timer 节点：`seconds` 字段
- group 节点：`loop_count` 字段
- http 节点：`timeout` 和 `expect_status` 字段

**实现方式**：
```vue
<!-- 旧实现 -->
<input v-model.number="delayConfig.ms" type="number" />

<!-- 新实现 -->
<input v-model="delayConfig.ms" type="text" placeholder="数值或 ${variable} 引用" />
```

**配套修改**：
- reactive 初始值从数字改为字符串（如 `'5000'`、`'200'`）
- 读取配置时使用 `String()` 转换，确保显示原始值
- 保存时直接序列化字符串，保持变量引用不被破坏

**理由**：
- 导入 YAML 后，GUI 能正确显示带变量引用的配置值
- 用户可以在 GUI 中编辑变量引用，而不会被强制转为数字
- 保持向后兼容：纯数字字符串仍可正常解析

**替代方案**：
- 仅修改 delay 节点 → 其他节点仍有同样问题
- 添加专门的变量编辑器 → 过度设计，增加复杂度

### Decision 8: card.yaml 重构

**选择**：将所有 think_time 替换为 delay 节点

**替换规则**：
```yaml
# 旧格式
- name: 连接模拟器
  type: http
  config:
    method: POST
    url: "..."
  think_time: "3s"

# 新格式
- name: 连接模拟器
  type: http
  config:
    method: POST
    url: "..."

- name: 等待模拟器响应
  type: delay
  config:
    ms: 3000
```

**edges 调整**：
```yaml
# 旧 edges
- from: 连接模拟器
  to: 空闲未插枪

# 新 edges
- from: 连接模拟器
  to: 等待模拟器响应
- from: 等待模拟器响应
  to: 空闲未插枪
```

**理由**：
- 符合 Salvo 架构（独立 delay 节点）
- 不引入节点级 think_time 能力（避免复杂度）
- delay 节点可复用、可配置、可监控

## Risks / Trade-offs

### Risk 1: 统一后处理影响性能

**风险**：每个节点都经过后处理逻辑，可能增加延迟

**缓解**：
- 只在有 extract/retry 配置时执行后处理
- 后处理逻辑轻量（JSON path 提取、简单计算）
- 通过 benchmark 验证性能影响

### Risk 2: Retry 退避时间过长

**风险**：指数退避可能导致总等待时间过长

**缓解**：
- 提供 max_backoff 上限配置
- 文档说明退避策略的使用场景
- 提供日志记录每次重试的等待时间

### Risk 3: While generator step 复杂度增加

**风险**：stepConfig 结构变复杂，维护成本增加

**缓解**：
- 保持向后兼容（Type 为空时默认 http）
- 添加单元测试覆盖所有 step 类型
- 文档说明 generator step 的使用方式

### Risk 4: card.yaml 重构引入错误

**风险**：替换 think_time 时可能遗漏或配置错误

**缓解**：
- 编写脚本自动化替换（而非手动修改）
- 替换后运行完整测试验证流程
- 保留原始 YAML 作为备份

### Trade-off 1: 统一后处理 vs 分散处理

**选择**：统一后处理

**代价**：
- sceneNode.Execute 逻辑变复杂
- 需要解析多种配置（extract、retry）

**收益**：
- 所有节点类型自动获得能力
- 代码复用，维护成本低
- 与 step 级实现保持一致

### Trade-off 2: 扁平 multipart vs 嵌套 form

**选择**：同时支持两种格式

**代价**：
- 需要自动推断 fields/files
- 可能推断错误（如字段值包含 "/"）

**收益**：
- 向后兼容 card.yaml
- 降低配置复杂度（简单场景）

## Migration Plan

### Phase 1: 实现统一后处理（1-2 天）

1. 在 runner.go 中添加 parseExtractConfig、parseRetryConfig 方法
2. 在 sceneNode.Execute 中添加后处理逻辑
3. 实现 calculateBackoff、applyExtract 方法
4. 添加单元测试

### Phase 2: 实现 while generator step（0.5 天）

1. 扩展 stepConfig 结构
2. 实现 executeWhileStepGenerator 方法
3. 添加单元测试

### Phase 3: 实现 multipart 兼容（0.5 天）

1. 修改 executeHTTP 解析逻辑
2. 添加单元测试

### Phase 4: 重构 card.yaml（0.5 天）

1. 编写替换脚本或手动替换 think_time
2. 调整 edges
3. 运行完整测试验证

### Rollback Strategy

- 每个 Phase 独立提交，可单独回滚
- 保留原始 card.yaml 备份
- 统一后处理不影响现有 while/parallel/loop 逻辑

## Open Questions

1. **extract 配置位置**：extract 应该放在 config 内还是节点级？
   - 当前方案：节点级（与 think_time、retry 一致）
   - 待确认：card.yaml 中 generator 的 extract 在 config 内，是否需要调整？

2. **retry 触发条件**：除了 HTTP 状态码，还需要支持哪些重试条件？
   - 当前方案：on_status（HTTP 状态码）
   - 待确认：是否需要 on_error（错误类型）、on_timeout 等？

3. **delay 节点格式**：是否支持 "3s"、"100ms" 等人类可读格式？
   - 当前方案：只支持毫秒数字（ms: 3000）
   - 待确认：是否需要扩展 delay 节点支持字符串格式？
