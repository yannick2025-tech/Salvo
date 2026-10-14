# Split Report Detail Table

## Why

测试报告列表接口加载时间超过 10 秒，严重影响用户体验。根因是 `reports` 表的 `detail` 字段包含大量数据（时序数据、节点指标等），导致查询性能极差。项目尚未上线，无历史数据负担，现在是进行架构优化的最佳时机。

## What Changes

- **BREAKING**: 拆分 `reports` 表的 `detail` 字段到独立的 `report_details` 表
- **BREAKING**: 改造 `ListReports` API，不再返回 `detail` 字段（性能目标：< 1 秒）
- 新增轻量级列表接口，只返回基本元数据和 `summary` 字段
- 改造 `GetReport` API，通过 JOIN 查询获取完整数据
- 数据库 Schema Migration：创建 `report_details` 表并迁移现有数据
- 前端适配：列表页使用轻量接口，详情页使用完整接口
- 新增智能预加载机制：前端预加载当前页前 5 个报告的详情数据

## Capabilities

### New Capabilities

- `report-list-optimization`: 轻量级报告列表接口，性能优化到 < 1 秒，不返回大字段数据
- `report-detail-preloading`: 智能预加载机制，前端预加载当前页前 N 个报告的详情数据，提升用户体验

### Modified Capabilities

- `report-detail-storage`: 数据存储结构从单表拆分为双表（reports + report_details），Detail 字段独立存储，支持高效查询

## Impact

### 数据库层
- `reports` 表：移除 `detail` 字段
- 新增 `report_details` 表：一对一关联 `reports` 表，存储 `detail` 字段
- Migration 脚本：创建新表、迁移数据、删除旧字段

### API 层
- `internal/store/sqlite/sqlite.go`: 改造所有 Report 相关查询逻辑
- `internal/api/handler.go`: 
  - `ListReports`: 只查询 `reports` 表（不含 detail）
  - `GetReport`: JOIN `report_details` 表获取完整数据
  - `ExportReport`: JOIN 查询获取 detail 数据
- `internal/api/dto/dto.go`: 
  - `ReportDTO`: 保持不变（前端兼容）
  - 新增 `ReportListItemDTO`: 轻量级列表项（不含 detail）

### 前端层
- `web/app/src/api/report.ts`: 
  - 新增 `listReportsLightweight()` 函数
  - 改造 `getReport()` 调用逻辑
- `web/app/src/views/reports/ReportsPage.vue`: 
  - 使用轻量级列表接口
  - 添加智能预加载逻辑（预加载前 5 个报告详情）
  - 新增内存缓存管理
- `web/app/src/views/reports/ReportDetailPage.vue`: 无需改动

### 测试
- 数据库 Migration 测试
- API 性能测试（目标：< 1 秒）
- 前端预加载逻辑测试