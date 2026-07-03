## Why

业务配置文件 card.yaml 使用了多种 YAML 特性，但当前 Salvo DAG runner 的顶层节点执行逻辑不支持部分特性。While/Parallel/Loop 的 step 已支持 extract 和 retry，但顶层 HTTP/Generator 节点缺少这些能力，导致业务 YAML 无法正常运行。同时，card.yaml 中的 think_time 应替换为 delay 节点以符合现有架构。

## What Changes

- **统一 extract 支持**：在 sceneNode.Execute 层统一处理 extract 后处理，使所有节点类型（HTTP、Generator 等）自动支持从响应中提取变量
- **节点级 retry 支持**：在 sceneNode.Execute 层统一处理 retry，支持指数退避策略（initial_backoff、multiplier、max_backoff、jitter）
- **While 步骤支持 generator 类型**：扩展 stepConfig 支持 `type: generator`，允许 while 循环内执行 generator 节点
- **multipart 格式兼容**：支持扁平格式 `multipart: {token: xxx, file: xxx}` 和嵌套格式 `form: {fields: {}, files: {}}`
- **card.yaml 重构**：将所有 think_time 替换为 delay 节点，移除 think_time 依赖

## Capabilities

### New Capabilities
- `unified-node-postprocessing`: 统一的节点后处理机制，包括 extract 提取和 retry 重试，在 sceneNode.Execute 层实现
- `while-generator-step`: While 步骤支持 generator 节点类型，扩展 stepConfig 结构

### Modified Capabilities
- `csv-data-source`: 无规格变更，仅实现细节
- `node-group`: 无规格变更，仅实现细节

## Impact

**代码影响**：
- `internal/runner/runner.go`：sceneNode.Execute 添加统一后处理逻辑
- `internal/runner/while_node.go`：stepConfig 添加 Type/Config 字段，executeWhile 支持 generator 类型
- `internal/protocol/http/http.go`：multipart 格式兼容

**YAML 影响**：
- `docs/biz-migration/card.yaml`：11 处 think_time 替换为 delay 节点，edges 相应调整

**向后兼容**：
- 现有 while/parallel/loop 的 step 级 extract/retry 保持不变
- 新增的节点级 extract/retry 是增量能力，不影响现有配置
