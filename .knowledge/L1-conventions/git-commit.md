---
layer: L1
maturity: proven
last-verified: 2026-05-25
source: .trae/skills/gater-conventions/SKILL.md
tags: [git, commit, conventional-commits]
---

# Git Commit Message 规范

> 所有代码变更的提交消息必须遵循此规范。

## 格式

- **语言**：英文
- **格式**：Conventional Commits（不带 scope）

```
<type>: <description>

- Detail line 1
- Detail line 2
- Detail line 3
```

## Type 列表

| Type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `refactor` | 代码重构（不改变功能） |
| `docs` | 文档变更 |
| `style` | 代码格式调整（不影响逻辑） |
| `test` | 测试相关 |
| `chore` | 构建/工具/依赖变更 |
| `perf` | 性能优化 |

## 规则

- **不使用 scope**：写 `fix:` 而非 `fix(web):`
- description 首字母小写，不加句号
- 使用祈使句（imperative mood）：`add` 而非 `added`
- Body 使用 `- ` 列表，每条详细描述做了什么、为什么这样做
- 列表项可跨行，续行缩进 2 空格对齐
- 单次 commit 只做一件事，避免混合多种 type

## 示例

```
fix: auto-restore running test session on page refresh

- On page mount, fetch sessions first then auto-select the session
  with testStatus=running and isOnline=true
- This restores the charging panel, stop button, and charging info
  polling after a page refresh
- Watch selectedSession changes to keep isTestRunning in sync
  with backend testStatus from session list polling
```

```
fix: correct auth_test to match actual bytesToBCD and computeAuthHash behavior

- Fix TestBytesToBCD: expected values should be hex ASCII strings,
  not mathematical BCD decoded bytes (bytesToBCD converts bytes to
  uppercase hex ASCII per protocol requirement, verified with charger)
- Fix TestComputeAuthHash_AlgorithmSteps: add hex.DecodeString for
  fixedKey to match computeAuthHash step 2 logic
- These test failures were pre-existing, not introduced by recent changes
```

## 提交约束

- **禁止自动提交**：完成代码变更后，只生成 commit message 并展示给用户，**绝不能自动执行 `git commit`**，必须等待用户确认后才可提交
- 用户说"生成 git message"或"请生成 commit message"时，只输出 message 内容，不执行 git 操作
