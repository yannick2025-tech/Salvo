---
layer: L4
maturity: proven
last-verified: 2026-05-25
source: .trae/skills/gater-conventions/SKILL.md
tags: [checklist, code-review, self-check, lint, quality]
---

# 变更后自检清单

> 每次完成代码变更后，按此清单逐项检查。

## CodeGraph 同步

- [ ] 修改前已通过 `SearchCodebase` 查找并理解相关代码（参考 [codegraph-workflow.md](./codegraph-workflow.md)）
- [ ] 修改后已执行 `codegraph sync` 同步索引

## 代码质量

- [ ] 代码能编译通过（`go build ./...` / `npm run build`）
- [ ] 无 linter 错误（`go vet ./...` / `npm run lint`）
- [ ] 无明显逻辑错误
- [ ] 错误处理完善（不吞错误、不遗漏 error 返回值）

## 功能正确性

- [ ] 变更符合需求描述
- [ ] 边界条件已处理
- [ ] 无硬编码的测试值残留

## 代码风格

- [ ] 符合 [coding-style.md](../L1-conventions/coding-style.md) 规范
- [ ] 新增导出符号有文档注释
- [ ] 无不必要的 `fmt.Println` / `console.log` 调试语句
- [ ] 数值格式化遵循 [number-formatting.md](../L1-conventions/number-formatting.md)
- [ ] ECharts 配置遵循 [chart-style.md](../L1-conventions/chart-style.md)
- [ ] UI 间距遵循 [ui-spacing.md](../L1-conventions/ui-spacing.md)

## Git 提交

- [ ] 变更分类正确（feat/fix/refactor/docs 等），参考 [git-commit.md](../L1-conventions/git-commit.md)
- [ ] Commit message 遵循 Conventional Commits 格式
- [ ] Commit message 使用英文
- [ ] 单次提交范围合理，不混合不相关变更

## 文档影响评估（必做）

> AI 完成评估并列出结论，由用户确认是否需要更新。即使评估结论为"无需更新"也须明确记录。

- [ ] 已逐项评估本次变更对以下文档的影响，并给出结论（无需更新 / 需更新）：
  - **OpenSpec specs**：`openspec/specs/` 下受影响的 capability spec，行为是否与变更后实现一致
  - **OpenSpec 变更文档**：若当前处于 `openspec/changes/xxx/` 变更流程中，`proposal.md` / `design.md` / `tasks.md` 是否需要同步修订
  - **API 文档 / 接口契约**：请求/响应结构、路由、错误码是否变化
  - **用户文档**：`docs/` 下的指南、配置说明、README 是否需要更新
  - **知识库**：`.knowledge/` 中与变更相关的约定是否需要补充或修订
- [ ] 如需更新：已列出待更新文档清单，交由用户确认
- [ ] 如用户确认更新：已修改对应文档并通过用户审查
