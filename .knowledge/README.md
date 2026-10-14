# Salvo Knowledge Base

> 结构化知识库，按层级组织。使用 `knowledge-loader` Skill 按需加载。

## 知识层级

| 层级 | 名称 | 语义 | 约束强度 | 加载时机 |
|------|------|------|---------|---------|
| **L0** | 个人偏好 | "我喜欢这样" | 无约束 | 不自动加载 |
| **L1** | 团队约定 | "我们必须这样" | 强约束 | 任务匹配时加载 |
| **L2** | 业务知识 | "系统是这样的" | 中约束 | 相关业务时加载 |
| **L3** | 项目知识 | "我们踩过这些坑" | 弱约束 | 排查问题时加载 |
| **L4** | 工作流 | "任务应该这样做" | 流程约束 | 执行流程时加载 |

## 文件索引

### L1-conventions/ (团队约定 - 必须遵守)

| 文件 | 内容 | 触发关键词 |
|------|------|-----------|
| [coding-style.md](./L1-conventions/coding-style.md) | Go + Vue/TS 代码风格 | 代码、函数、注释、命名 |
| [git-commit.md](./L1-conventions/git-commit.md) | Git 提交规范 | commit、message、提交 |
| [number-formatting.md](./L1-conventions/number-formatting.md) | 数值格式化（百分比/QPS/延迟） | toFixed、小数、百分比、QPS、ms |
| [chart-style.md](./L1-conventions/chart-style.md) | ECharts 图表样式规范 | chart、echarts、图表、曲线、tooltip |
| [ui-spacing.md](./L1-conventions/ui-spacing.md) | UI 间距规范 | margin、padding、间距、布局 |

### L2-business/ (业务知识 - 预留)

| 文件 | 内容 | 状态 |
|------|------|------|
| load-testing-domain.md | 负载测试领域模型 | 预留 |
| api-contracts.md | API 契约与数据格式 | 预留 |
| data-model.md | 数据模型与存储设计 | 预留 |

### L3-project/ (项目知识 - 参考性质)

| 文件 | 内容 | 触发关键词 |
|------|------|-----------|
| [pitfalls.md](./L3-project/pitfalls.md) | 已知陷阱（踩坑经验） | bug、问题、陷阱、踩坑 |
| [debugging-playbook.md](./L3-project/debugging-playbook.md) | 排查手册（方法论模板） | 排查、调试、定位、卡顿、溢出 |

### L4-workflows/ (工作流 - 流程约束)

| 文件 | 内容 | 触发关键词 |
|------|------|-----------|
| [codegraph-workflow.md](./L4-workflows/codegraph-workflow.md) | CodeGraph 使用流程（修改前搜索、修改后同步） | codegraph、搜索、sync、修改代码、SearchCodebase |
| [code-review-checklist.md](./L4-workflows/code-review-checklist.md) | 变更后自检清单 | 自检、审查、checklist、lint |
| bug-fix-sop.md | Bug 修复 SOP | 预留 |
| feature-dev-sop.md | 功能开发 SOP | 预留 |

## 成熟度标签

每篇文档头部包含元数据：

```
layer: L1
maturity: proven       # draft → verified → proven
last-verified: 2026-05-25
source: project_rules.md
tags: [...]
```

| 标签 | 含义 | 审查频率 |
|------|------|---------|
| `draft` | 初稿，未经充分验证 | 2 周内验证或归档 |
| `verified` | 经 1-2 次项目验证 | 月度审查 |
| `proven` | 经 3+ 次项目验证 | 季度审查 |

## 使用方式

1. 不要一次性读取所有文件
2. 根据当前任务类型，使用 `knowledge-loader` Skill 加载对应文件
3. L1 约定类优先级最高，涉及代码修改时必须遵守
4. L3 陷阱类在排查问题时参考，修复后考虑是否新增 Lesson

## 来源映射

本知识库从以下原始来源重组：

| 原始来源 | 提取目标 |
|---------|---------|
| `.trae/rules/project_rules.md` | L1: number-formatting, chart-style, ui-spacing, coding-style; L3: debugging-playbook |
| `.trae/skills/gater-conventions/SKILL.md` | L1: git-commit, coding-style; L4: code-review-checklist |
| `docs/lessons-learned.md` | L3: pitfalls |
