## Why

Dashboard 全局指标（P50/P99 延迟、QPS、Pending Queue、错误率）在导入默认 YAML 以时长模式运行后全部显示 0 或异常值，导出报告同样为 0。根因是 6 个独立 bug 叠加：默认 YAML base_url 指向不存在域名、TimeSeriesCollector 的 runID 与 run_record ID 不匹配导致时序数据查询永远为空、loadSystemMetricsFromDB 硬编码 TaskWait P50/P95 为 0、场景停止后 Dashboard 仍读 live stats 导致延迟变化、全局 P50 聚合逻辑只取百分位点而非原始延迟。这些问题使整个 Dashboard 和报告系统在默认配置下不可用，必须在上线前修复。

## What Changes

- **修复** `example-v2.yaml` 的 `base_url` 变量链，将 `env` 从 `staging` 改为 `local`，`api_host` 指向 `localhost:9090`，与 `example-full-coverage.yaml` 和 `salvo.yaml` 保持一致
- **修复** `Runner` 构造函数中 `TimeSeriesCollector` 的 runID 与 `run_record` ID 不匹配问题：只调用一次 `n.Generate()`，共享给 collector 和 run_record
- **修复** `loadSystemMetricsFromDB` 中 `TaskWaitP50Ms` 和 `TaskWaitP95Ms` 硬编码为 0 的问题，从 summary 结构正确读取
- **修复** Dashboard 轮询逻辑：runner 停止后（`Status() != running`）改用 `run_records` 存储值，不再读 live stats，消除延迟值变化
- **修复** 全局 P50 聚合逻辑：对于运行中的场景，从 live stats 获取原始延迟列表（`GetAllLatencies`）而非仅取 P50/P90/P99 三个百分位点
- **验证并修复** 失败请求在报告和 HTML 报告中的展示：确认 `failed_nodes` 字段正确序列化和渲染

## Capabilities

### New Capabilities

无新增 capability，所有修改均针对已有 capability 的 bug 修复。

### Modified Capabilities

- `runtime-metrics-collector`: 修复 PendingQueue TaskWait P50/P95 在 loadSystemMetricsFromDB 中硬编码为 0 的问题，确保从 summary 正确读取
- `report-export`: 修复 TimeSeriesCollector runID 与 run_record ID 不匹配导致报告时序数据为空的问题；验证 failed_nodes 字段正确存储和渲染
- `html-report-export`: 修复 HTML 报告中时序数据（QPS/P50/P95/P99）为空的问题，确保失败请求详情正确展示

## Impact

### 配置文件
- `configs/example-v2.yaml`: 修改 `env` 和 `api_host` 变量，指向本地 mock server

### 后端
- `internal/runner/runner.go`: 修复 `New()` 函数中 runID 重复生成问题（L294/L299）；修复 Dashboard 轮询停止后延迟变化的逻辑
- `internal/api/handler.go`: 修复 `loadSystemMetricsFromDB` 中 P50/P95 硬编码（L2364-L2365）；修复全局 P50 聚合逻辑（L2118-L2120）

### 测试
- 需新增/更新测试：验证 runID 一致性、loadSystemMetricsFromDB 返回正确 P50/P95、停止后 Dashboard 读存储值
