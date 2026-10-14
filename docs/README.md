# Salvo 文档导航

本目录包含 Salvo 性能测试平台的各类文档。

## 📚 文档结构

### 功能与使用
- [功能清单](features.md) - 平台完整功能列表（前端菜单 + 后端模块）

### 设计文档
详见 [design/](design/) 目录：

#### 核心架构
- [Salvo 整体设计](design/2026-04-30-salvo-design-zh.md) - 系统架构、核心组件、执行引擎
- [Web UI 与 RBAC 设计](design/2026-05-02-web-ui-rbac-design.md) - 前端界面、权限控制
- [生成器与条件分支设计](design/2026-05-03-generator-selector-ifelse-design.md) - 数据生成、条件逻辑

#### 模块设计
- [加密模块设计](design/crypto.md) - AES-CBC/GCM 加密解密
- [SO 插件架构](design/so-plugin-architecture.md) - 插件系统、热加载机制
- [生成器模块设计](design/generator.md) - JSON Schema 数据生成
- [测试报告系统设计](design/2026-05-07-enhanced-test-report-system-design.md) - 报告生成、导出
- [雪花 ID 精度分析](design/snowflake-precision-analysis.md) - JavaScript 大数精度问题

#### 知识管理
- [知识分层架构设计](design/knowledge-architecture-design.md) - `.knowledge/` 知识库设计

### 业务迁移
详见 [biz-migration/](biz-migration/) 目录：
- [YAML 场景配置指南](biz-migration/salvo-yaml-guide.md) - DAG 编排、节点类型、变量系统
- [业务迁移设计](biz-migration/design.md) - 迁移方案

### 历史归档
详见 [archive/](archive/) 目录：
- [实施计划](archive/implementation-plan.md) - 已完成的项目实施计划
- [实施计划（中文）](archive/implementation-plan-zh.md) - 中文版实施计划

## 🧠 知识库

团队约定、已知陷阱、工作流等详见 [.knowledge/](../.knowledge/) 目录：

- **L1 约定**：代码风格、图表样式、数值格式化、Git 提交规范
- **L3 项目知识**：已知陷阱、调试手册
- **L4 工作流**：代码审查清单

## 📝 文档维护

- 设计文档统一放在 `docs/design/` 目录
- 历史/已完成文档放在 `docs/archive/` 目录
- 业务知识放在 `docs/biz-migration/` 目录
- 团队约定和陷阱放在 `.knowledge/` 目录
