## Why

当前 Runner 在执行节点时，无论节点是否失败都会继续执行后续节点。HTTP 非 2xx 响应、业务错误（如 JSON 解析失败）、以及 while 循环中的步骤失败都不会阻断链条执行。这导致：
1. 错误被静默忽略，问题难以定位
2. 后续节点可能因为前置节点失败而执行无意义操作（如使用空变量发起支付）
3. 无法区分"可跳过的非关键节点"和"必须成功的核心节点"

需要引入节点级别的错误阻断配置，让场景配置者可以精确控制失败行为。

## What Changes

- 在节点配置中新增 `block_on_error` 字段（布尔值，默认 `false`）
  - `false`（默认）：节点失败后记录错误，继续执行后续节点
  - `true`：节点失败后立即中断整个 chain 执行，标记为失败
- 修改 `sceneNode` 结构体，支持从 YAML/JSON 配置解析 `block_on_error` 字段
- 修改 DAG executor 执行逻辑，在节点失败后根据 `block_on_error` 决定是否继续执行
- 修改 while 循环步骤配置，支持步骤级别的 `block_on_error` 控制
- 保持向后兼容：未配置时默认为 `false`，现有场景行为不变

## Capabilities

### New Capabilities
- `node-error-blocking`: 节点级别的错误阻断控制能力，支持在节点配置中指定失败时是否中断整个 chain 执行

### Modified Capabilities
（无现有 spec 需要修改）

## Impact

**受影响的代码：**
- `internal/runner/runner.go`：节点执行后错误处理逻辑
- `internal/runner/while_node.go`：while 循环步骤错误处理
- `internal/core/dag/executor.go`：DAG 执行器节点失败传播逻辑
- `internal/store/model/node.go`：节点模型新增 `block_on_error` 字段
- `internal/store/migrations/`：数据库迁移脚本
- `internal/api/node_handler.go`：节点 API 支持新字段
- 前端节点编辑表单（可选，后续迭代）

**受影响的 API：**
- 节点创建/更新 API 支持 `block_on_error` 字段
- 场景配置 YAML 解析支持新字段

**依赖：**
- 无外部依赖变更
- 数据库需要新增字段（ALTER TABLE nodes ADD COLUMN block_on_error BOOLEAN DEFAULT FALSE）
