# unified-error-handling — Design

## Context

当前 handler 层错误处理是逐处手写的 `fmt.Sprintf` 透传模式：

- `internal/api/auth_handler.go`：15 处（login/create user/get user/update/delete/roles 等）
- `internal/api/handler.go`：45 处（scenes/nodes/runs/reports 等核心业务）
- `internal/api/so_handler.go`：10 处

错误分类现状：仅 `sql.ErrNoRows` 部分映射为 404；其余全部 500 + 原始错误文本。`repo.ErrEmailTaken`（本次软删除修复引入）是首个哨兵错误先例：store 层识别 SQLite 约束错误 → handler 映射 409 + 中文提示。

项目响应协议：`dto.Response{Code, Message, Data}`，`Code=0` 成功；handler 返回 Response 由统一 writeJSON 写出。

## Goals / Non-Goals

Goals:
- 5xx 响应永不携带原始错误（脱敏），原始错误完整落日志
- 可预期业务错误按语义返回 4xx + 中文提示
- 一次性替换全部 70 处透传模式
- 保持现有 `Code=400/404` 行为不回退

Non-Goals:
- 不引入错误码体系改造（如全局业务错误码枚举、i18n）——响应协议 {Code, Message} 不变
- 不改前端错误展示逻辑
- 不重构 store 层错误包装（仅在已识别的业务冲突点补哨兵）

## Decisions

### D1. helper 形态：handler 包内函数 `internalErr(op string, err error) dto.Response`

放 `internal/api/errors.go`。签名取 `op`（操作名，如 "create user"）用于日志定位。

理由：Handler 方法已是 `func(r *http.Request) dto.Response`，用函数替代散落的 sprintf，最小侵入。

### D2. 错误映射规则（三档）

| 错误类型 | 判定 | 响应 |
|---------|------|------|
| 请求/参数错误 | 现有 decode/校验逻辑 | 400（不变） |
| 可预期业务错误 | `errors.Is` 匹配哨兵（ErrEmailTaken / ErrNotFound 等） | 4xx + 中文提示 |
| 未知错误 | 其余一切 | 500 + "服务器内部错误"，日志记 op + 原始错误 |

### D3. 哨兵错误范围：本轮只补已识别的业务冲突

- `repo.ErrEmailTaken`（已有）→ 409 "邮箱已被使用"
- `sql.ErrNoRows` → 沿用现有 404 处理（helper 内统一：ErrNoRows → 404 "资源不存在"）
- 其余业务冲突（场景名重复等）**不在本轮扩**，遇到时按同模式增量补

理由：避免一次性枚举全部业务错误的遗漏风险；哨兵是增量演进模式。

### D4. 日志：未知错误记 logger.Error（op + err），响应不含 err 文本

helper 内部调用项目 logger。可预期业务错误（4xx）记 Warn 级别或不记（无系统异常）。

### D5. 保留 400 参数类处理不动

decode 失败、字段校验的现有 `ErrorResp(400, ...)` 是请求方问题，信息本身面向用户可读，不纳入 internalErr。

### D6. 替换策略：机械替换 + 保持 Message 语义

70 处中：`sql.ErrNoRows` 已显式 404 的保留原逻辑或收敛进 helper（`getByIdNotFound` 分支统一）；纯 `fmt.Sprintf("op: %v", err)` 的直接换 `internalErr("op", err)`。替换由测试保障：现有测试断言 Code 不变（500/404），新增测试断言 5xx Message 为统一文案。

## Risks / Trade-offs

- **排查成本**：5xx Message 不再含原始错误 → 用日志补偿（D4），op 前缀保证定位。低风险。
- **测试 Message 断言**：若现有测试断言了 500 Message 的具体技术文本，会红 → 实施时同步修正断言为统一文案（属预期变更）。
- **70 处替换的回归风险** → 三个文件分批替换，每批跑全量测试；Code 值全部保持不变。

## Migration Plan

1. errors.go helper + 单测
2. auth_handler.go 15 处（含已有 ErrEmailTaken 映射收敛进 helper）
3. so_handler.go 10 处
4. handler.go 45 处（最大，单独一批）
5. 全量回归 + 前端冒烟（错误场景 Message 展示）
