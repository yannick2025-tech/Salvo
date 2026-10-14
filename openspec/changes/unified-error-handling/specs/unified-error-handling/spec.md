# unified-error-handling — Spec

## ADDED Requirements

### Requirement 1: 系统错误统一脱敏响应

任何不可预期错误（数据库故障、内部异常）的 API 响应 SHALL 返回 `Code=500`、`Message="服务器内部错误"`，SHALL NOT 包含原始错误文本、表名、约束名或内部路径。

#### Scenario: 数据库写入失败

- **WHEN** store 层返回非哨兵错误（如磁盘 I/O 错误）
- **THEN** 响应 Code=500、Message="服务器内部错误"
- **AND** 原始错误（含操作名）写入服务端日志

#### Scenario: 约束冲突未识别为业务错误

- **WHEN** INSERT 触发未映射哨兵的 UNIQUE 约束
- **THEN** 响应 Code=500、Message="服务器内部错误"（不泄漏 `UNIQUE constraint failed: ...`）

### Requirement 2: 可预期业务错误语义映射

可预期业务错误 SHALL 通过哨兵错误识别，映射为对应 4xx 与中文提示。

#### Scenario: 邮箱被占用

- **WHEN** 创建用户时 store 返回 `repo.ErrEmailTaken`
- **THEN** 响应 Code=409、Message="邮箱已被使用"

#### Scenario: 资源不存在

- **WHEN** 查询的资源 ID 不存在（`sql.ErrNoRows`）
- **THEN** 响应 Code=404、Message 不含技术细节

### Requirement 3: 排查可观测性不降级

5xx 统一文案 SHALL NOT 降低可观测性：helper MUST 以 Error 级别记录操作名与完整原始错误。

#### Scenario: 排查线上 5xx

- **WHEN** 某接口返回 500
- **THEN** 服务端日志含操作名前缀与原始错误堆栈文本，可据此定位

## UNCHANGED Requirements

### Requirement 4: 请求参数错误保持现状

- decode 失败与字段校验错误 SHALL 继续返回 `Code=400` 及现有可读提示，不纳入统一脱敏（面向用户的信息本身可读且无害）
