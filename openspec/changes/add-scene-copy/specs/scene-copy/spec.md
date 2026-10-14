# scene-copy 变更规格

## ADDED Requirements

### Requirement: 场景复制 API

系统 SHALL 提供 `POST /api/v1/scenes/copy` 接口，接收 `{scene_id: string, name: string}`，在单个数据库事务内创建一个内容与源场景完全一致的新场景，并返回新场景的 SceneDTO（ID 以 string 序列化）。

#### Scenario: 完整复制

- **WHEN** 请求复制一个包含 3 个节点、2 条连线、1 个变量、1 个 CSV 数据源（含全部 rows）的场景
- **THEN** 新场景的 nodes、edges、data_sources 行数与源场景一致，节点 type/config/position、连线 condition/priority、变量值、CSV rows 逐字段相等，且新场景所有实体使用新的 snowflake ID

#### Scenario: 复制名称语义

- **WHEN** 请求中 name 为非空字符串
- **THEN** 新场景 name 等于该字符串，status 为 `draft`，dag_json 为空，description 与源场景完全一致（原样继承，不添加任何标记）

#### Scenario: 源场景不存在

- **WHEN** scene_id 不对应任何未删除场景
- **THEN** 返回 400 错误，不创建任何数据

#### Scenario: 新名称已存在

- **WHEN** name 与现有未删除场景重名
- **THEN** 返回与 CreateScene 相同语义的冲突错误（409），不创建任何数据

#### Scenario: 名称缺失

- **WHEN** 请求中 name 为空或缺失
- **THEN** 返回 400 错误，不创建任何数据

### Requirement: 复制的事务原子性

复制过程 SHALL 在单个 SQLite 事务内完成场景本体、nodes、edges、data_sources 的全部写入；任一步失败时整体回滚，SHALL NOT 残留任何部分复制的实体。

#### Scenario: 中途失败回滚

- **WHEN** 源场景存在悬空边（from/to 引用不存在的节点）或 group 节点 config.node_ids 含未知节点 ID，导致映射失败
- **THEN** 事务回滚，返回 500 错误，数据库中不存在新场景及其任何 nodes/edges/data_sources 行

### Requirement: 复制时重写节点 ID 引用

复制 SHALL 维护源节点 ID 到新节点 ID 的映射，并据此重写 `edges.from_node_id/to_node_id` 以及 group 节点 `config.node_ids`；新场景中的所有节点引用 SHALL 仅指向新场景自身的节点。

#### Scenario: 连线引用重写

- **WHEN** 源场景存在节点 A→B 的连线
- **THEN** 新场景存在对应新节点 A'→B' 的连线，A'、B' 为 A、B 的新 ID，连线 condition/priority 与源一致

#### Scenario: group 节点引用重写

- **WHEN** 源场景存在 group 节点且其 config.node_ids 引用同场景的 2 个子节点
- **THEN** 新场景的 group 节点 config.node_ids 指向这 2 个子节点的新 ID，不引用源场景任何节点

#### Scenario: 名称引用保持不变

- **WHEN** 节点 config 中通过名称引用变量（`${var}`）或数据源（dsName）
- **THEN** 复制后节点 config 中这些名称引用逐字节保持不变

### Requirement: 复制不包含运行历史

复制 SHALL NOT 复制源场景的 run_records（运行记录）与测试报告；新场景的运行历史从空开始。

#### Scenario: 运行历史隔离

- **WHEN** 源场景已有 2 条运行记录和 1 份测试报告
- **THEN** 复制后新场景的运行记录数为 0、报告数为 0，源场景的运行记录与报告保持不变

### Requirement: 前端复制入口跳转场景列表

场景编辑页的"复制"按钮 SHALL 调用场景复制 API；复制成功后系统 SHALL 跳转到场景列表页。复制失败时 SHALL 停留在当前页并展示后端错误信息，允许用户修改名称重试。

#### Scenario: 复制成功跳转

- **WHEN** 用户在复制弹窗中输入合法新名称并确认，API 返回成功
- **THEN** 系统展示成功提示并导航至场景列表页，列表中可见新场景

#### Scenario: 复制失败可重试

- **WHEN** 用户输入的名称与现有场景重名，API 返回冲突错误
- **THEN** 系统展示后端返回的错误信息，页面保持在场景编辑页，用户可修改名称后重新提交

#### Scenario: 复制结果可编辑可运行

- **WHEN** 用户从场景列表进入新复制场景的编辑页并直接启动测试
- **THEN** DAG 画布展示与源场景一致的节点与连线，测试按源场景的配置执行
