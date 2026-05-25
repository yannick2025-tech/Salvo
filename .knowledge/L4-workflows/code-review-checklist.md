---
layer: L4
maturity: proven
last-verified: 2026-05-25
source: .trae/skills/gater-conventions/SKILL.md
tags: [checklist, code-review, self-check, lint, quality]
---

# 变更后自检清单

> 每次完成代码变更后，按此清单逐项检查。

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

## 文档（如适用）

- [ ] 相关文档已同步更新
- [ ] 新增功能有对应文档说明
- [ ] 示例代码可运行
