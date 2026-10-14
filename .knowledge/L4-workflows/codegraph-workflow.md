---
layer: L4
maturity: proven
last-verified: 2026-06-16
source: project_rules.md
tags: [codegraph, workflow, search, sync, code-modification]
---

# CodeGraph 工作流

> 项目已集成 CodeGraph（基于嵌入模型的代码语义索引）。每次修改代码**必须**遵循以下流程，确保修改前充分理解上下文、修改后索引保持同步。

## 修改前：SearchCodebase 查找相关代码

在动手修改任何代码之前，**必须先使用 `SearchCodebase` 工具搜索相关代码**，理解现有实现后再做变更。

### 使用原则

| 场景 | 搜索策略 | 示例 |
|------|---------|------|
| 修改某函数/方法 | 搜索该函数的实现和所有调用点 | "Where is `handleRequest` implemented and called?" |
| 新增功能 | 搜索类似功能的已有实现 | "How does the existing metrics collection work?" |
| 修复 Bug | 搜索相关模块的数据流 | "How is user session validated in the middleware?" |
| 重构代码 | 搜索受影响的所有引用 | "Where are `Config` struct fields accessed?" |

### 注意事项

- 用**完整的自然语言问题**搜索，而非关键词堆砌
- 先广度搜索（不限定目录），确认范围后再缩小到目标目录
- 一次只问一个问题，复杂问题拆分为多次搜索
- **禁止跳过此步骤直接修改代码**

## 修改后：CodeGraph Sync 同步索引

完成代码修改后，**必须执行 `codegraph sync`** 将变更同步到 CodeGraph 索引。

### 执行时机

- 每次代码编辑完成（Edit / Write）后
- Git commit 前
- 关闭工作区 / 切换任务前

### 执行方式

```bash
codegraph sync
```

### 为什么必须同步

CodeGraph 索引基于磁盘文件实时构建。不同步会导致：
- 后续 `SearchCodebase` 搜不到最新修改的代码
- 语义索引过期，搜索结果不准确
- 团队成员无法通过索引发现你的变更

## 流程总结

```
┌─────────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ 1. SearchCodebase   │ ──▶ │ 2. 编辑代码       │ ──▶ │ 3. codegraph sync │
│    查找相关代码       │     │    (Edit/Write)   │     │    同步索引        │
└─────────────────────┘     └──────────────────┘     └─────────────────┘
         ▲                                                      │
         │                      ┌──────────────┐               │
         └──────────────────────│ 4. 验证/自检   │◀──────────────┘
                                │ (checklist)   │
                                └──────────────┘
```
