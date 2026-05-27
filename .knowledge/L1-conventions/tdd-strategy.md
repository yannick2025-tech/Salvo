---
layer: L1
maturity: proven
last-verified: 2026-05-26
source: docs/superpowers/specs/2026-04-30-salvo-design-zh.md §13
tags: [tdd, testing, coverage, unit-test, integration-test, e2e]
---

# TDD 策略

> 所有代码变更应遵循测试驱动开发流程，确保质量可验证。

## 测试金字塔

- **70% 单元测试** — 接口模拟实现，表驱动测试
- **20% 集成测试** — 模块间交互（DAG+协程池+变量）
- **10% 端到端测试** — 完整场景 HTTP 测试

## 测试工具链

| 工具 | 用途 |
|------|------|
| testing | 标准库，表驱动测试 |
| testify/assert | 流式断言 |
| testify/mock | 接口模拟 |
| testcontainers | MySQL/PG 集成测试 |
| httptest | HTTP 处理器测试 |
| go test -race | 竞态检测 |

## 覆盖率目标

≥ 80% 行覆盖率

## TDD 工作流

1. **红灯** — 先写失败测试
2. **绿灯** — 最小实现使测试通过
3. **重构** — 重构，测试保持绿灯
4. **覆盖率** — 验证覆盖率达标
