# unified-error-handling — Tasks

## 1. errors helper（TDD）

- [x] 1.1 写测试（红→绿）：internalErr 未知错误 → 500 + "服务器内部错误"（不含 err 文本）；ErrEmailTaken → 409 "邮箱已被使用"；sql.ErrNoRows → 404 "资源不存在"
- [x] 1.2 实现 internal/api/errors.go：internalErr(op, err) —— 哨兵映射 + 未知 500 + logger.Error(op + 原始错误)。实现形态为 Handler 方法（h.log 可直接使用，设计 D1 微调）
- [x] 1.3 测试转绿（3 用例）

## 2. auth_handler.go 替换（15 处）

- [x] 2.1 全部 `ErrorResp(500, fmt.Sprintf("xxx: %v", err))` 换为 h.internalErr("xxx", err)；ErrEmailTaken 分支收敛进 helper（CreateUser 单行调用）；清理未用 import（fmt/errors）
- [x] 2.2 api 包全量测试绿（10.8s），Code 断言不变
- [x] 2.3 用户业务错误映射测试已在 errors_test.go + auth_user_role_test.go（409/404 路径）覆盖

## 3. so_handler.go 替换（10 处）

- [x] 3.1 替换 + 测试绿

## 4. handler.go 替换（45 处）

- [x] 4.1 全部替换（含 4 处 %q 双参变体：create/get/update group node、create data source → h.internalErr(fmt.Sprintf("op %s", name), err)）+ 测试绿
- [x] 4.2 场景类业务冲突点检查：本轮无新增哨兵需求（ErrNoRows 已有显式 404 分支保留；场景名等冲突点未发现既有约束依赖，后续增量补）

## 5. 收尾

- [x] 5.1 go test ./... 全量回归零失败 + go vet 通过 + codegraph sync
- [x] 5.2 知识库评估：pitfalls.md 新增 Lesson 10（软删除×唯一约束冲突 + 错误透传两层教训）；debugging-playbook.md 触发条件表补索引行
- [x] 5.3 汇报：替换统计（3 文件 70 处）、响应行为变化、commit message 建议
