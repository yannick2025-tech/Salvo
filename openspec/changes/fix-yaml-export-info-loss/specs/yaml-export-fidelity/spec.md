## ADDED Requirements

### Requirement: YAML 导出为后端单一可信源
"复制 YAML"与"导出 YAML"操作 SHALL 调用后端 `POST /api/v1/scenes/export` 获取 YAML 文本，前端 SHALL NOT 维护独立的 YAML 序列化逻辑（删除 `generateYaml` 及其辅助函数）。

#### Scenario: 复制 YAML 调用后端导出
- **WHEN** 用户在场景编辑页点击"复制 YAML"
- **THEN** 前端 SHALL 调用 `exportYAML(sceneId)` 并将返回的 YAML 文本写入剪贴板（带 textarea+execCommand 兜底）
- **AND** 前端 SHALL NOT 使用本地 `generateYaml()` 生成内容

#### Scenario: 导出 YAML 调用后端导出
- **WHEN** 用户点击"导出 YAML"下载文件
- **THEN** 前端 SHALL 调用 `exportYAML(sceneId)` 并将返回的 YAML 作为 `.yaml` 文件下载
- **AND** 文件内容 SHALL 与"复制 YAML"剪贴板内容完全一致（同一 API 结果）

#### Scenario: 导出过程显示加载与错误态
- **WHEN** 导出 API 请求进行中
- **THEN** 触发按钮 SHALL 显示加载态且禁用重复点击
- **WHEN** 导出 API 返回非 0 code 或网络失败
- **THEN** 前端 SHALL 提示"导出失败，请重试"且不写入剪贴板/不下载文件

### Requirement: YAML 导出无信息丢失
后端 `/scenes/export` SHALL 输出完整场景结构，包括：`name`、`description`、`default_timeout`、`variables`、`config_params`、`derived_params`、`data_sources`、`setup`、`nodes`、`teardown`、`edges`，以及每个节点的 `block_on_error`。`yaml.Marshal` SHALL 保证缩进与引号正确。

#### Scenario: 导出含全部 scene 级字段
- **WHEN** 场景设置了 name、description、default_timeout、variables、config_params、derived_params
- **THEN** 导出 YAML SHALL 包含 `name`、`description`、`default_timeout`、`variables`、`config_params`、`derived_params` 顶层键且值一致

#### Scenario: 导出含 block_on_error
- **WHEN** 节点的 `block_on_error` 为 true
- **THEN** 导出 YAML 中该节点 SHALL 包含 `block_on_error: true`
- **WHEN** 节点的 `block_on_error` 为 false
- **THEN** 导出 YAML SHALL 省略该字段（`omitempty`）

#### Scenario: 导出 setup/teardown 分节
- **WHEN** 场景存在 lifecycle 为 `setup` 或 `teardown` 的节点
- **THEN** 导出 YAML SHALL 将它们分别置于 `setup:`/`teardown:` 顶层段，main 段节点置于 `nodes:`

### Requirement: YAML 导入导出往返一致
对同一 YAML 文档执行"导入→导出→再导入"后，节点数量、类型、config、lifecycle 分节、edges、variables 三分类、data_sources SHALL 保持一致。

#### Scenario: 往返保留 setup/teardown 分节
- **WHEN** 原始 YAML 含 `setup:`/`teardown:` 段，段内节点真实 type 为 generator/http
- **THEN** 导入后节点 `lifecycle` 分别为 `setup`/`teardown`
- **AND** 导出 YAML SHALL 将这些节点还原到 `setup:`/`teardown:` 段（而非混入 `nodes:`）
- **AND** 再导入后分节与节点数不变

#### Scenario: 往返保留 block_on_error 与 default_timeout
- **WHEN** 原始 YAML 节点含 `block_on_error: true` 且场景含 `default_timeout: N`
- **THEN** 导入→导出→再导入后，对应节点 `block_on_error` 仍为 true，场景 `default_timeout` 仍为 N

#### Scenario: 往返保留 variables/config_params/derived_params
- **WHEN** 原始 YAML 含 `variables`、`config_params`（`cfg_` 前缀键）、`derived_params`（`${...}` 值）
- **THEN** 导入→导出后，三者 SHALL 按后端启发式还原到对应段，键值一致
