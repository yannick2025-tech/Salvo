# Salvo 知识分层架构设计

> 借鉴 Harness Engineering 知识管理实践，为 Salvo 项目构建结构化、可检索、可演进的知识体系。

## 一、背景与动机

### 1.1 行业趋势

Harness Engineering（驾驭层工程）是 2026 年 AI 工程领域的核心方法论。其核心公式：

```
Agent = Model + Harness
```

在 Harness 的三支柱（上下文工程、架构约束、持续治理）中，**知识管理本身就是核心能力**，而非附属品。正如腾讯技术工程团队的实践总结：

> "工作流只是管道，知识才是流过管道的活水。模型会迭代，工具链会更新，工作流会重构。但团队积累的领域模型、架构决策、最佳实践、已知陷阱——这些知识是永恒的。"

### 1.2 Salvo 现状问题

| 问题 | 表现 | 影响 |
|------|------|------|
| **知识混杂** | 编码规范、图表样式、排查方法论全塞在一个 `project_rules.md` 里 | LLM 每次加载全量，上下文浪费 |
| **无分层** | 团队约定和业务知识混在一起，无法按需检索 | AI Agent 无法区分"必须遵守"和"参考了解" |
| **无成熟度** | 所有知识同等对待，没有 draft/verified/proven 标签 | 无法判断知识的可信程度 |
| **缺失领域** | 业务知识、架构决策、工作流 SOP 完全空白 | AI Agent 缺乏业务上下文，容易犯语义错误 |
| **无演进机制** | 知识只增不减，没有衰减和归档流程 | 文档膨胀，过时知识污染上下文 |

### 1.3 目标

1. **结构化**：将知识按层级、类型、成熟度三维正交组织
2. **可检索**：LLM 能根据任务类型按需加载对应知识，避免上下文爆炸
3. **可演进**：知识有生命周期，从 draft 到 proven，过时知识可归档
4. **渐进式**：不破坏现有约束体系，增量建设

---

## 二、方案对比

### 2.1 知识存储位置

| 方案 | 描述 | 优势 | 劣势 | 评价 |
|------|------|------|------|------|
| **A. 仓库内 `.knowledge/`** | 知识目录放在项目仓库内 | 与代码同版本；LLM 直接访问；零额外配置 | 知识与代码耦合 | ✅ 推荐 |
| B. 独立 Git 仓库 | 知识库是独立仓库 | 知识可跨项目复用；职责分离 | 需额外 clone；LLM 访问路径复杂；单项目过度设计 | ❌ 过重 |
| C. `.trae/` 子目录 | 放在 `.trae/knowledge/` | 与现有 Trae 配置对齐 | `.trae/` 是 IDE 配置，语义不符；知识不应绑定特定 IDE | ❌ 语义不当 |

**选择方案 A 的理由**：
- Salvo 是单项目，不需要跨项目知识复用
- 仓库内目录 LLM 可直接用 Read 工具访问，无需额外配置
- `.knowledge/` 语义清晰，与 `.trae/`（IDE 配置）职责分明

### 2.2 知识加载机制

| 方案 | 描述 | 优势 | 劣势 | 评价 |
|------|------|------|------|------|
| **A. Rules 指向 + Skill 按需加载** | Rules 加知识地图索引（~30行），Skill 按任务类型加载 | Rules 轻量不占上下文；Skill 精准检索；渐进式披露 | 需新建 Skill | ✅ 推荐 |
| B. 纯 Rules | 所有知识塞进 `project_rules.md` | 简单，LLM 自动加载 | 上下文爆炸；文件膨胀到数千行；LLM 被淹没 | ❌ 不可扩展 |
| C. 纯 Skill | 知识完全由 Skill 管理，Rules 不提及 | 灵活 | LLM 不知道知识库存在，不会主动触发 Skill | ❌ 可见性差 |

**选择方案 A 的理由**：
- **渐进式披露**：Rules 只放索引（地图），Skill 按需加载详情（路线），符合 Harness Engineering 的核心原则
- **上下文效率**：LLM 每次会话只看到 ~30 行索引，而非数千行全文
- **精准检索**：Skill 根据任务关键词匹配对应知识文件，只加载需要的
- **可扩展**：新增知识只需加文件 + 更新索引，不影响 Rules 体积

### 2.3 现有内容处理

| 方案 | 描述 | 优势 | 劣势 | 评价 |
|------|------|------|------|------|
| **A. 保留原文件，`.knowledge/` 作为重组版** | 原文件不动，`.knowledge/` 是结构化重组 | 不破坏现有引用；平滑过渡 | 两份数据源需保持同步 | ✅ 推荐 |
| B. 迁移后删除原文件 | 内容迁移到 `.knowledge/` 后删除原文件 | 单一数据源 | 需更新所有引用路径；风险高 | ❌ 破坏性大 |
| C. 仅新增，不重组 | `.knowledge/` 只放缺失的知识 | 零风险 | 约定类知识仍然混杂 | ❌ 不解决核心问题 |

**选择方案 A 的理由**：
- `.trae/rules/project_rules.md` 被 Trae IDE 自动加载，删除会破坏现有工作流
- `.trae/skills/gater-conventions/` 被多个 Skill 引用，删除影响面大
- 重组版在 `.knowledge/` 中以细粒度文件组织，AI Agent 按需加载更精准
- 同步策略：`.knowledge/` 是"结构化重组版"，原文件是"原始来源"，以 `.knowledge/` 为准

### 2.4 文档粒度

| 方案 | 描述 | 优势 | 劣势 | 评价 |
|------|------|------|------|------|
| **A. 细粒度文件** | 每个主题独立文件 | AI Agent 按需检索精准；上下文效率高；易于维护 | 文件数量多 | ✅ 推荐 |
| B. 粗粒度文件 | 每层一个文件 | 文件少 | 单文件过长；无法精准加载 | ❌ 上下文浪费 |
| C. 混合粒度 | L1 拆细，L2-L4 粗 | 折中 | 规则不统一 | ⚠️ 可选 |

**选择方案 A 的理由**：
- 细粒度是 Harness Engineering 渐进式披露的核心要求
- AI Agent 只需加载 `chart-style.md` 而非整个 `project_rules.md`
- 文件数量多不是问题，因为 Skill 负责按需检索，LLM 不需要手动浏览

---

## 三、最终设计方案

### 3.1 目录结构

```
.knowledge/                              ← 知识根目录
├── README.md                            ← 知识地图（目录索引，~100行）
│
├── L0-preferences/                      ← 个人偏好（.gitignore，不提交）
│   └── .gitkeep
│
├── L1-conventions/                      ← 团队约定（强约束，AI 必须遵守）
│   ├── coding-style.md                  ← 代码风格（Go + Vue/TS）
│   ├── git-commit.md                    ← Git 提交规范
│   ├── number-formatting.md             ← 数值格式化规范（百分比、QPS、延迟）
│   ├── chart-style.md                   ← ECharts 图表样式规范
│   └── ui-spacing.md                    ← UI 间距规范
│
├── L2-business/                         ← 业务知识（领域模型、业务流程）
│   ├── load-testing-domain.md           ← 负载测试领域模型（预留）
│   ├── api-contracts.md                 ← API 契约与数据格式（预留）
│   └── data-model.md                   ← 数据模型与存储设计（预留）
│
├── L3-project/                          ← 项目知识（架构决策、已知陷阱）
│   ├── architecture.md                  ← 架构决策记录（预留）
│   ├── pitfalls.md                      ← 已知陷阱（从 lessons-learned 提取）
│   ├── debugging-playbook.md            ← 排查手册（从 project_rules 提取）
│   └── tech-decisions.md               ← 技术选型记录（预留）
│
└── L4-workflows/                        ← 工作流知识（SOP、任务模板）
    ├── bug-fix-sop.md                   ← Bug 修复 SOP（预留）
    ├── feature-dev-sop.md               ← 功能开发 SOP（预留）
    └── code-review-checklist.md         ← 代码审查清单
```

### 3.2 知识层级定义

| 层级 | 名称 | 语义 | 约束强度 | 更新频率 | 示例 |
|------|------|------|---------|---------|------|
| **L0** | 个人偏好 | "我喜欢这样" | 无约束，纯个人 | 随时 | 编辑器配置、主题偏好 |
| **L1** | 团队约定 | "我们必须这样" | 强约束，必须遵守 | 低（季度级） | 代码风格、Git 规范、数值格式化 |
| **L2** | 业务知识 | "系统是这样的" | 中约束，理解必须 | 中（月级） | 领域模型、API 契约、数据模型 |
| **L3** | 项目知识 | "我们踩过这些坑" | 弱约束，参考性质 | 高（周级） | 架构决策、已知陷阱、排查手册 |
| **L4** | 工作流 | "任务应该这样做" | 流程约束 | 中（月级） | Bug 修复 SOP、代码审查清单 |

### 3.3 成熟度标签

每篇知识文档头部必须包含元数据：

```markdown
---
layer: L1
maturity: proven       # draft → verified → proven
last-verified: 2026-05-25
source: project_rules.md
tags: [formatting, percentage, echarts]
---
```

| 成熟度 | 含义 | 可信度 | 使用场景 |
|--------|------|--------|---------|
| `draft` | 初稿，未经充分验证 | ⚠️ 低 | 新发现的知识，待验证 |
| `verified` | 经 1-2 次项目验证 | ✅ 中 | 团队内确认的知识 |
| `proven` | 经 3+ 次项目验证，写入测试 | ✅✅ 高 | 可信赖的规范 |

### 3.4 LLM 加载机制

#### 3.4.1 Rules 层：知识地图索引

在 `project_rules.md` 顶部添加 ~30 行知识地图：

```markdown
## Knowledge Index

项目知识库位于 `.knowledge/`，按层级组织。根据任务类型按需加载：

| 任务类型 | 加载文件 | 层级 |
|---------|---------|------|
| 修改代码风格/格式 | `.knowledge/L1-conventions/coding-style.md` | L1 |
| 修改图表/ECharts | `.knowledge/L1-conventions/chart-style.md` | L1 |
| 修改数值显示 | `.knowledge/L1-conventions/number-formatting.md` | L1 |
| 修改 UI 间距 | `.knowledge/L1-conventions/ui-spacing.md` | L1 |
| Git 提交 | `.knowledge/L1-conventions/git-commit.md` | L1 |
| 排查 Bug | `.knowledge/L3-project/debugging-playbook.md` + `pitfalls.md` | L3 |
| 新增 API | `.knowledge/L2-business/api-contracts.md` | L2 |
| 新增功能 | `.knowledge/L4-workflows/feature-dev-sop.md` | L4 |

> 使用 knowledge-loader Skill 按需加载，不要一次性读取所有文件。
```

#### 3.4.2 Skill 层：knowledge-loader

新建 `.trae/skills/knowledge-loader/SKILL.md`，核心逻辑：

```markdown
# knowledge-loader

根据任务类型，按需加载 `.knowledge/` 中对应的知识文件。

## 触发规则

当用户任务匹配以下关键词时，自动加载对应知识文件：

| 关键词模式 | 加载文件 |
|-----------|---------|
| 图表/chart/echarts/曲线 | L1-conventions/chart-style.md |
| 格式化/小数/toFixed/百分比 | L1-conventions/number-formatting.md |
| 排查/bug/调试/溢出/卡顿 | L3-project/debugging-playbook.md, L3-project/pitfalls.md |
| 提交/commit/message | L1-conventions/git-commit.md |
| API/接口/路由 | L2-business/api-contracts.md |
| 新功能/feature/需求 | L4-workflows/feature-dev-sop.md |

## 加载策略

1. 优先加载 L1（约定），因为是最强约束
2. 根据任务类型加载 L2-L4 中的相关文件
3. 同一层级内，只加载相关文件，不加载整层
4. 加载后检查 maturity 标签，draft 级别的知识需提示用户确认
```

#### 3.4.3 加载流程

```
用户发起任务
    ↓
Rules 中的知识地图告知 LLM 知识库存在
    ↓
LLM 根据任务类型匹配关键词
    ↓
触发 knowledge-loader Skill
    ↓
Skill 读取对应 .knowledge/ 文件
    ↓
LLM 获得精准上下文，执行任务
```

### 3.5 知识生命周期管理

```
发现新知识 → draft → 项目验证 → verified → 多项目验证 → proven
                ↓                        ↓
              归档/删除              定期审查（季度）
```

**衰减规则**：
- `proven` 知识：每季度审查一次，确认是否仍然适用
- `verified` 知识：每月审查一次
- `draft` 知识：2 周内未验证则归档或删除

**归档目录**：
```
.knowledge/
└── _archive/          ← 归档的过时知识
    └── YYYY-MM-DD-name.md
```

---

## 四、现有内容映射

### 4.1 从 project_rules.md 提取

| 原始章节 | 目标文件 | 层级 |
|---------|---------|------|
| Number & Percentage Formatting | `L1-conventions/number-formatting.md` | L1 |
| Chart Conventions (Axis Labels, Tooltip Precision) | `L1-conventions/chart-style.md` | L1 |
| ECharts Line Chart Style | `L1-conventions/chart-style.md` | L1 |
| UI Spacing | `L1-conventions/ui-spacing.md` | L1 |
| Go Code Style | `L1-conventions/coding-style.md` | L1 |
| Backend Data | `L2-business/data-model.md` | L2 |
| Problem Solving Workflow | `L3-project/debugging-playbook.md` | L3 |

### 4.2 从 gater-conventions 提取

| 原始章节 | 目标文件 | 层级 |
|---------|---------|------|
| Git Commit Message 规范 | `L1-conventions/git-commit.md` | L1 |
| 代码风格 | `L1-conventions/coding-style.md` | L1 |
| 变更后自检清单 | `L4-workflows/code-review-checklist.md` | L4 |

### 4.3 从 lessons-learned.md 提取

| 原始内容 | 目标文件 | 层级 |
|---------|---------|------|
| Lesson 1: 前端首屏性能优化 | `L3-project/pitfalls.md` | L3 |
| Lesson 2: 链路跟踪溢出 | `L3-project/pitfalls.md` | L3 |
| 排查方法论 | `L3-project/debugging-playbook.md` | L3 |

### 4.4 预留目录（暂无内容）

| 文件 | 层级 | 预期内容 | 优先级 |
|------|------|---------|--------|
| `L0-preferences/` | L0 | 个人偏好配置 | 低（暂空） |
| `L2-business/load-testing-domain.md` | L2 | 负载测试领域模型、术语表 | 中 |
| `L2-business/api-contracts.md` | L2 | API 契约、请求/响应格式 | 中 |
| `L3-project/architecture.md` | L3 | 架构决策记录（ADR） | 中 |
| `L3-project/tech-decisions.md` | L3 | 技术选型记录 | 低 |
| `L4-workflows/bug-fix-sop.md` | L4 | Bug 修复标准流程 | 中 |
| `L4-workflows/feature-dev-sop.md` | L4 | 功能开发标准流程 | 低 |

---

## 五、实施计划

### Phase 1：基础建设（本次实施）

1. 创建 `.knowledge/` 目录结构
2. 编写 `README.md`（知识地图）
3. 从现有文件提取并重组 L1 约定类知识（5 个文件）
4. 从 lessons-learned 提取 L3 项目知识（2 个文件）
5. 从 gater-conventions 提取 L4 工作流知识（1 个文件）
6. 新建 `knowledge-loader` Skill
7. 更新 `project_rules.md` 添加知识地图索引

### Phase 2：业务知识填充（后续）

1. 编写 `L2-business/load-testing-domain.md`
2. 编写 `L2-business/api-contracts.md`
3. 编写 `L2-business/data-model.md`

### Phase 3：工作流完善（后续）

1. 编写 `L4-workflows/bug-fix-sop.md`
2. 编写 `L4-workflows/feature-dev-sop.md`
3. 编写 `L3-project/architecture.md`

### Phase 4：知识治理（持续）

1. 建立知识审查周期
2. 实施成熟度标签管理
3. 定期归档过时知识

---

## 六、预期收益

| 维度 | 现状 | 改进后 |
|------|------|--------|
| **上下文效率** | 每次加载 ~450 行 project_rules.md | 按需加载 ~50-80 行精准知识 |
| **知识覆盖** | 只有约定和陷阱，缺业务和架构 | 五层全覆盖 |
| **可维护性** | 单文件混杂，改一处影响全局 | 细粒度文件，独立维护 |
| **AI 准确性** | 缺乏业务上下文，容易犯语义错误 | 按需注入领域知识 |
| **知识演进** | 只增不减，无生命周期 | 成熟度标签 + 衰减归档 |
| **团队协作** | 知识散落在个人脑中 | 结构化共享，新成员快速上手 |

---

## 七、参考来源

- [Harness不是目的，知识才是护城河 — 腾讯技术工程](https://zhuanlan.zhihu.com/p/2042008174581003025)
- [Harness Engineering 全景指南](https://www.cnblogs.com/chengxin1985/p/19933892)
- [Anthropic: Building effective agents](https://www.anthropic.com/engineering/building-effective-agents)
- [OpenAI Codex: 100万行代码实践](https://openai.com/index/codex/)
