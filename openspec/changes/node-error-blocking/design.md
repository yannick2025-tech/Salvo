## Context

当前 Runner 的节点错误处理存在两个问题：
1. **HTTP 非 2xx 响应不返回 error**：`executeHTTP()` 在 HTTP 请求失败时返回 `&dag.Output{Error: err}, nil`，不阻断链条
2. **节点执行 error 会阻断链条**：`executor.go` 中节点返回 error 后，子节点会被跳过（`parentFailed` 逻辑），但这是隐式行为，无法配置

需要在节点级别提供显式的 `block_on_error` 配置，让场景配置者精确控制失败行为。

## Goals / Non-Goals

**Goals:**
- 在节点配置中新增 `block_on_error` 布尔字段，默认 `false`
- `block_on_error: false`：节点失败后记录错误，继续执行后续节点（当前行为）
- `block_on_error: true`：节点失败后立即取消整个 chain 执行，标记场景为失败
- 支持所有节点类型（http, generator, while, if-else, delay 等）
- while 循环内的步骤也支持独立的 `block_on_error` 配置
- 向后兼容：未配置时默认 `false`，现有场景行为不变

**Non-Goals:**
- 不支持"跳过当前节点但继续执行无关节点"的细粒度控制（未来可扩展）
- 不修改前端节点编辑表单（后续迭代）
- 不修改 while 循环的 `fail_after_consecutive` 机制（保持独立）

## Decisions

### Decision 1: 在 `dag.Node` 接口新增 `BlockOnError() bool` 方法

**选择**：扩展 `dag.Node` 接口，新增 `BlockOnError() bool` 方法。

**理由**：
- `sceneNode` 已实现 `dag.Node` 接口，新增方法自然融入现有架构
- DAG executor 可直接通过接口方法判断，无需类型断言
- 默认返回 `false`，保持向后兼容

**替代方案**：
- 在 `dag.Output` 中增加 `BlockOnError` 字段 → 不够清晰，error 传播逻辑复杂
- 在 `dag.Executor` 中维护节点配置 map → 增加复杂度，不如接口方法直接

### Decision 2: HTTP 非 2xx 响应和 `expect_body` 断言失败时返回 error

**选择**：
1. 在 `executeHTTP()` 中，当 `block_on_error: true` 且 HTTP 状态码非 2xx 时，返回 error
2. 修复 `expect_body` 断言失败时返回 error（当前返回 `Output{Error}, nil`，应改为 `nil, error`）

**理由**：
- 当前 HTTP 非 2xx 不返回 error，导致错误被静默忽略
- 当前 `expect_body` 断言失败返回 `Output{Error}, nil`，不会触发 error 处理逻辑
- 通过 `block_on_error` 控制是否将非 2xx 视为 error，保持灵活性
- `expect_body` 失败应视为节点执行失败，配合 `block_on_error` 可阻断 chain

**实现细节**：
```go
// executeHTTP 中 - HTTP 非 2xx
if !httpResp.IsSuccess() && n.blockOnError {
    return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, body)
}

// executeHTTP 中 - expect_body 断言失败（修复）
if !exists || actualVal != expectedVal {
    // 修复前：return &dag.Output{Error: fmt.Errorf("%s", errMsg)}, nil
    // 修复后：
    return nil, fmt.Errorf("expect_body validation failed: %s", errMsg)
}
```

### Decision 3: 在 DAG executor 中通过 context 取消实现 chain 阻断

**选择**：当节点 `BlockOnError() == true` 且执行失败时，调用 `context.Cancel()` 取消整个 chain。

**理由**：
- 当前 executor 已有 context 取消处理逻辑
- 通过 context 取消可以优雅地停止所有正在执行的节点
- 避免修改 executor 核心逻辑，降低风险

**实现细节**：
```go
// executor.go Execute() 中
output, err := n.Execute(nodeCtx, input)
if err != nil {
    if n.BlockOnError() {
        // 取消整个 chain
        cancel()  // 需要 executor 持有 cancel 函数
    }
    errCh <- fmt.Errorf("node %s execute: %w", n.ID(), err)
    // ...
}
```

### Decision 4: 数据库新增 `block_on_error` 列

**选择**：在 `nodes` 表新增 `block_on_error BOOLEAN DEFAULT FALSE` 列。

**理由**：
- 简单直接，与现有字段（如 `timeout`, `loop_count`）保持一致
- 默认值 `FALSE` 保证向后兼容
- 迁移脚本可独立执行

### Decision 5: while 循环步骤支持独立的 `block_on_error`

**选择**：在 `stepConfig` 中新增 `BlockOnError bool` 字段，控制步骤失败时是否中断 while 循环。

**理由**：
- while 循环的步骤与 DAG 节点是不同层级的概念
- 步骤失败时，可以选择中断整个 while 循环（视为节点失败）
- 如果 while 节点本身 `block_on_error: true`，则中断整个 chain

**实现细节**：
```go
// while_node.go 中
if stepErr != nil {
    if step.BlockOnError {
        // 中断 while 循环，返回 error
        return nil, fmt.Errorf("step %q failed and block_on_error is true", step.Name)
    }
    // 否则继续执行后续步骤
}
```

## Risks / Trade-offs

**[Risk] 默认行为改变导致现有场景异常**
→ Mitigation: `block_on_error` 默认 `false`，现有场景无需修改，行为完全不变

**[Risk] context 取消后资源清理不完整**
→ Mitigation: 现有代码已有 context 取消处理逻辑（`select { case <-ctx.Done(): }`），无需额外修改

**[Risk] while 循环步骤 `block_on_error` 与 `fail_after_consecutive` 冲突**
→ Mitigation: 两者独立工作：
- `fail_after_consecutive`：连续失败 N 次后中断
- `block_on_error`：单次失败即中断
- 优先级：`block_on_error` > `fail_after_consecutive`

**[Risk] 数据库迁移失败**
→ Mitigation: 使用 `ALTER TABLE ADD COLUMN`，SQLite 支持该操作，失败可回滚

**[Trade-off] 不支持"跳过当前节点但继续执行无关节点"**
→ 当前设计只支持"阻断整个 chain"或"继续执行"两种模式
→ 未来可扩展：新增 `on_error: "skip"` 选项，跳过当前节点但继续执行无关节点
