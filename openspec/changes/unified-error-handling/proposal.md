# unified-error-handling

## Why

后端异常直接透传给前端，没有任何统一处理与脱敏：

1. **原始错误外泄**：handler 层存在 70 处 `dto.ErrorResp(500, fmt.Sprintf("xxx: %v", err))` 模式（auth_handler.go 15 处、handler.go 45 处、so_handler.go 10 处），把 SQLite 原始错误（如 `UNIQUE constraint failed: user.email`）、内部错误细节原样展示给用户
2. **体验差**：用户看到的技术性错误信息无法理解，也无行动指引
3. **安全隐患**：泄漏内部表名、约束结构、错误路径
4. **业务错误无分类**：可预期的业务冲突（邮箱占用等）与不可预期的系统错误混在 500，前端无法按状态码做差异化处理

## What Changes

- **handler 层统一错误映射 helper**：`internalErr(op string, err error) dto.Response`
  - 已知业务错误（哨兵/分类错误）→ 对应 4xx + 中文友好提示，**不含原始错误文本**
  - 未知错误 → 统一 500 + "服务器内部错误"，**原始错误只写入日志**（含 op 上下文）
- **store 层哨兵错误体系**：预定义可预期业务错误（`repo.ErrEmailTaken` 已是首个先例），SQLite 驱动错误在此层识别并转换
- **全量替换 70 处直接抛错模式**，覆盖 auth_handler / handler / so_handler 三个文件
- **日志规范**：未知错误必须记录 op + 完整原始错误，便于排查（不牺牲可观测性）

## Capabilities

- **错误分类响应**：API 层错误响应分三档——请求参数错误（400，已有）、可预期业务冲突（409/4xx，新增映射）、不可预期系统错误（500 统一文案）
- **内部信息脱敏**：任何 5xx 响应不包含数据库原始错误、内部路径、底层实现细节

## Impact

- **代码**：internal/api 三个 handler 文件（70 处错误响应）、新增 errors helper 文件、internal/store/repo（哨兵错误定义）
- **兼容性**：前端目前对非 0 Code 的处理为提示 Message——5xx 的 Message 从技术错误变为统一文案，前端展示改善，无需改动；依赖 5xx Message 内容的调用方（无）不受影响
- **测试**：现有断言 `ErrorResp(500, ...)` Code 的测试不受影响；新增 helper 单测 + 各业务错误映射测试
