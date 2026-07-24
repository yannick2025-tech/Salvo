# Split Report Detail Table - Design

## Context

### Current State

当前 `reports` 表包含 `detail` 字段，存储完整的测试报告数据（JSON 格式），包括：
- 全局统计指标（GlobalSummary）
- 全局时序数据（GlobalTimeSeries，可能包含几百到几千个样本点）
- 节点级指标（NodeMetrics，每个节点包含时序数据）
- 错误摘要、系统指标等

单个报告的 `detail` 字段大小通常在 100KB - 2MB 之间，某些复杂测试可能达到 5MB+。

### Problem

1. **性能问题**: `ListReports` 查询返回所有报告的完整数据，当报告数量较多时（如 50 个报告），响应体可能达到 50MB-100MB，加载时间超过 10 秒
2. **资源浪费**: 列表页只需要基本指标（总请求、成功率、延迟百分位），这些信息在 `summary` 字段中已有，无需加载 `detail` 字段
3. **扩展性差**: 随着测试报告数量增长，性能问题会越来越严重

### Constraints

- 项目尚未上线，无历史数据负担
- 不需要向后兼容，可以直接改造 API
- 列表页字段保持不变，只拆接口，不改变内容
- 性能目标：列表接口 < 1 秒

### Stakeholders

- **后端开发者**: 需要改造数据库 schema 和 API 逻辑
- **前端开发者**: 需要适配新的 API 接口和添加预加载逻辑
- **用户**: 期望列表页快速加载，详情页流畅体验

## Goals / Non-Goals

### Goals

1. **性能优化**: 将列表接口响应时间从 10+ 秒降到 < 1 秒（降低 90%+）
2. **数据架构优化**: 拆分大字段到独立表，提高查询效率
3. **用户体验提升**: 实现智能预加载，让大部分详情页秒开
4. **代码可维护性**: 保持代码结构清晰，便于后续扩展

### Non-Goals

1. **不改变列表页展示内容**: 列表字段保持不变（ID、场景、状态、总请求、成功率、P50/P95/P99、时间等）
2. **不优化详情页性能**: 详情页已经很快（单个报告查询），无需优化
3. **不添加新功能**: 不添加新的统计指标或图表功能

## Decisions

### Decision 1: 数据库 Schema 拆分方案

**选择**: 将 `reports` 表拆分为 `reports` + `report_details` 两张表

**方案对比**:

| 方案 | 优点 | 缺点 | 评估 |
|------|------|------|------|
| **方案 A: 拆分为独立表** (已选) | 查询性能最佳，架构清晰，未来可扩展性好 | 需要数据库迁移，改动较大 | ✅ 推荐：项目未上线，迁移成本低 |
| 方案 B: 保留原表，API 层优化 | 改动小，无需数据库迁移 | 性能提升有限（仍需扫描大字段），扩展性差 | ❌ 不推荐：治标不治本 |
| 方案 C: 使用 JSON 列类型 | 某些数据库支持部分 JSON 查询 | SQLite 支持有限，性能提升不明显 | ❌ 不推荐：依赖数据库特性 |

**Schema 设计**:

```sql
-- reports 表（轻量级元数据）
CREATE TABLE reports (
    id INTEGER PRIMARY KEY,
    scene_id INTEGER NOT NULL,
    run_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    summary TEXT,              -- 保留：包含基本统计指标
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

-- report_details 表（大字段独立存储）
CREATE TABLE report_details (
    report_id INTEGER PRIMARY KEY,  -- 一对一关联 reports.id
    detail TEXT,                     -- 完整的 ReportDetail JSON
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE
);

-- 索引优化
CREATE INDEX idx_reports_scene_id ON reports(scene_id);
CREATE INDEX idx_reports_run_id ON reports(run_id);
CREATE INDEX idx_reports_status ON reports(status);
CREATE INDEX idx_reports_started_at ON reports(started_at);
```

**为什么选择拆分方案**:
1. 查询性能最佳：列表查询只扫描小表，速度提升 10-100 倍
2. 数据架构清晰：大字段独立存储，符合数据库设计最佳实践
3. 未来可扩展性好：可对 `report_details` 表单独优化（如添加压缩、分表等）
4. 迁移成本低：项目未上线，无历史数据负担

### Decision 2: API 层改造方案

**选择**: 改造 `ListReports` 和 `GetReport`，不新增批量预加载接口

**方案对比**:

| 方案 | 优点 | 缺点 | 评估 |
|------|------|------|------|
| **方案 A: 改造现有接口** (已选) | 改动最小，API 简洁，前端适配简单 | 无 | ✅ 推荐：满足需求 |
| 方案 B: 新增批量预加载接口 | 减少请求数量（1 次批量 vs N 次单独） | 需要新增接口，增加维护成本 | ❌ 过度设计：当前方案已满足性能需求 |

**API 改造设计**:

```go
// ListReports: 轻量级列表接口
func (h *Handler) ListReports(r *http.Request) dto.Response {
    // 只查询 reports 表（不含 detail）
    reports, err := h.reports.List(r.Context(), repo.Filter{...})

    // 返回 ReportListItemDTO（不含 detail 字段）
    items := make([]dto.ReportListItemDTO, 0, len(reports))
    for _, rp := range reports {
        items = append(items, toReportListItemDTO(rp))
    }

    return dto.OK(dto.ListResponse[[]dto.ReportListItemDTO]{...})
}

// GetReport: 完整详情接口（JOIN 查询）
func (h *Handler) GetReport(r *http.Request) dto.Response {
    // JOIN report_details 表获取完整数据
    report, err := h.reports.GetByID(r.Context(), req.ID)
    // 返回完整 ReportDTO（含 detail）
    return dto.OK(toReportDTO(report))
}
```

**DTO 设计**:

```go
// ReportListItemDTO: 列表项（轻量级）
type ReportListItemDTO struct {
    ID         snowflake.ID `json:"id,string"`
    SceneID    snowflake.ID `json:"scene_id,string"`
    RunID      snowflake.ID `json:"run_id,string"`
    Status     string       `json:"status"`
    Summary    string       `json:"summary"`
    StartedAt  *time.Time   `json:"started_at,omitempty"`
    FinishedAt *time.Time   `json:"finished_at,omitempty"`
    CreatedAt  time.Time    `json:"created_at"`
    UpdatedAt  time.Time    `json:"updated_at"`
    // 注意：不包含 Detail 字段
}

// ReportDTO: 完整报告（用于详情页）
type ReportDTO struct {
    ID         snowflake.ID `json:"id,string"`
    SceneID    snowflake.ID `json:"scene_id,string"`
    RunID      snowflake.ID `json:"run_id,string"`
    Status     string       `json:"status"`
    Summary    string       `json:"summary"`
    Detail     string       `json:"detail"`     // 包含完整数据
    StartedAt  *time.Time   `json:"started_at,omitempty"`
    FinishedAt *time.Time   `json:"finished_at,omitempty"`
    CreatedAt  time.Time    `json:"created_at"`
    UpdatedAt  time.Time    `json:"updated_at"`
}
```

**为什么选择改造现有接口**:
1. 满足性能需求：列表接口 < 100ms，远低于 < 1s 的目标
2. 改动最小：只需改造两个接口，无需新增接口
3. 前端适配简单：只需调用新的 DTO 类型
4. API 简洁：避免接口爆炸

### Decision 3: 前端预加载机制

**选择**: 智能预加载当前页前 5 个报告详情

**方案对比**:

| 方案 | 优点 | 缺点 | 评估 |
|------|------|------|------|
| **方案 A: 智能预加载前 N 个** (已选) | 用户体验好，资源消耗可控，实现简单 | 未预加载的报告需额外等待 | ✅ 推荐：平衡体验和资源 |
| 方案 B: 后台预加载所有 | 用户点击任意报告都秒开 | 消耗大量带宽和内存，可能加载用户不看的数据 | ❌ 过度消耗：资源浪费严重 |
| 方案 C: 按需加载 | 资源消耗最低 | 用户每次点击都要等待加载 | ❌ 体验差：未优化用户体验 |

**预加载逻辑**:

```typescript
// ReportsPage.vue
const detailCache = new Map<string, ReportDTO>()

async function fetchReports() {
  // 1. 加载列表（轻量级）
  const resp = await listReports({ limit: 50 })
  reports.value = resp.data.items || []

  // 2. 智能预加载前 5 个报告详情
  const preloadCount = Math.min(5, reports.value.length)
  const preloadPromises = reports.value.slice(0, preloadCount).map(r =>
    getReport(r.id).then(resp => {
      detailCache.set(r.id, resp.data)
    }).catch(() => {/* 忽略预加载失败 */})
  )

  // 不等待预加载完成，让用户先看到列表
  Promise.all(preloadPromises)
}

// 点击详情时
async function viewReport(id: string) {
  // 优先使用缓存
  if (detailCache.has(id)) {
    // 立即跳转，无需等待
    router.push(`/reports/${id}`)
    return
  }

  // 缓存未命中，显示加载状态
  loading.value = true
  try {
    await getReport(id)
    router.push(`/reports/${id}`)
  } finally {
    loading.value = false
  }
}
```

**为什么选择智能预加载**:
1. 用户体验好：大部分用户点击前 5 个报告时秒开
2. 资源消耗可控：只预加载 5 个报告，不会过度消耗带宽
3. 实现简单：前端逻辑清晰，无需复杂的缓存管理
4. 平衡体验和资源：在用户体验和资源消耗之间取得平衡

### Decision 4: 数据迁移策略

**选择**: 一次性迁移，不保留旧 schema

**迁移步骤**:

```sql
-- Step 1: 创建 report_details 表
CREATE TABLE report_details (
    report_id INTEGER PRIMARY KEY,
    detail TEXT,
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE
);

-- Step 2: 迁移现有数据
INSERT INTO report_details (report_id, detail)
SELECT id, detail FROM reports WHERE detail IS NOT NULL;

-- Step 3: 删除 reports 表的 detail 字段
ALTER TABLE reports DROP COLUMN detail;

-- Step 4: 创建索引
CREATE INDEX idx_reports_scene_id ON reports(scene_id);
CREATE INDEX idx_reports_run_id ON reports(run_id);
CREATE INDEX idx_reports_status ON reports(status);
CREATE INDEX idx_reports_started_at ON reports(started_at);
```

**为什么选择一次性迁移**:
1. 项目未上线，无历史数据负担
2. 迁移成本低，无需保留旧 schema
3. 代码简洁，无需维护兼容逻辑

## Risks / Trade-offs

### Risk 1: 数据库迁移失败

**风险**: Migration 执行过程中可能失败，导致数据丢失或 schema 不一致

**缓解措施**:
- 在 Migration 前备份数据库
- 使用事务保证原子性（SQLite 支持）
- 编写 Migration 测试用例，验证数据完整性
- 提供 Rollback 脚本（恢复旧 schema）

### Risk 2: 前端缓存管理复杂度

**风险**: 前端预加载逻辑可能引入内存泄漏或缓存不一致问题

**缓解措施**:
- 使用 Vue 的 `onUnmounted` 钩子清理缓存
- 设置缓存大小上限（如最多缓存 10 个报告）
- 用户刷新页面时清空缓存
- 添加缓存命中率监控，优化预加载策略

### Risk 3: JOIN 查询性能下降

**风险**: `GetReport` 接口从单表查询变为 JOIN 查询，可能影响性能

**缓解措施**:
- 确保 `report_details.report_id` 是主键，查询效率高
- 添加性能测试，对比优化前后的响应时间
- 如果 JOIN 性能不理想，考虑使用两个独立查询（先查 reports，再查 report_details）

### Trade-off 1: 实现复杂度 vs 性能优化

**权衡**: 数据库拆分增加了实现复杂度，但带来了显著的性能提升

**分析**:
- 增加的复杂度：数据库 Migration、JOIN 查询逻辑
- 性能提升：列表接口从 10s+ 降到 < 100ms（提升 100 倍）
- **结论**: 性能提升远大于增加的复杂度，值得投入

### Trade-off 2: 预加载资源消耗 vs 用户体验

**权衡**: 预加载消耗带宽和内存，但提升了用户体验

**分析**:
- 预加载成本：每页加载 5 个报告详情（约 500KB-10MB）
- 用户体验提升：大部分详情页秒开，用户无需等待
- **结论**: 用户体验优先，资源消耗在可接受范围内（< 10MB）

## Migration Plan

### Phase 1: 数据库 Migration（优先级：高）

**步骤**:
1. 编写 Migration 脚本（创建表、迁移数据、删除字段）
2. 编写 Migration 测试用例（验证数据完整性）
3. 在测试环境执行 Migration，验证正确性
4. 备份数据库
5. 执行 Migration
6. 验证 Migration 结果（数据完整性、查询性能）

**Rollback 策略**:
```sql
-- 恢复 reports 表的 detail 字段
ALTER TABLE reports ADD COLUMN detail TEXT;

-- 从 report_details 恢复数据
UPDATE reports SET detail = (
    SELECT detail FROM report_details WHERE report_id = reports.id
);

-- 删除 report_details 表
DROP TABLE report_details;
```

### Phase 2: 后端 API 改造（优先级：高）

**步骤**:
1. 修改 `internal/store/model/model.go`，移除 `Report.Detail` 字段
2. 新增 `ReportDetail` model
3. 改造 `internal/store/sqlite/sqlite.go` 查询逻辑
4. 新增 `ReportListItemDTO`
5. 改造 `ListReports` 和 `GetReport` handler
6. 编写单元测试和集成测试
7. 性能测试（验证 < 1s 目标）

### Phase 3: 前端适配（优先级：高）

**步骤**:
1. 修改 `web/app/src/api/report.ts`，适配新的 API
2. 改造 `ReportsPage.vue`，使用轻量级列表接口
3. 实现智能预加载逻辑
4. 添加内存缓存管理
5. 编写前端测试用例
6. E2E 测试（验证用户体验）

### Phase 4: 测试和验证（优先级：高）

**步骤**:
1. 数据库 Migration 测试
2. API 性能测试（对比优化前后）
3. 前端预加载逻辑测试
4. E2E 测试（完整流程）
5. 性能基准测试（验证 < 1s 目标）

## Open Questions

### Q1: report_details 表是否需要添加 created_at/updated_at 字段？

**背景**: 当前设计中 `report_details` 表只有 `report_id` 和 `detail` 两个字段

**考虑**:
- 添加时间戳字段可以追踪数据变更历史
- 但增加了字段冗余（reports 表已有时间戳）

**建议**: 不添加，保持简洁。`report_details` 表只需要和 `reports` 表一对一关联即可。

### Q2: 预加载数量是否需要可配置？

**背景**: 当前设计固定预加载前 5 个报告

**考虑**:
- 不同用户的使用习惯可能不同（有的只看第一个，有的会浏览多个）
- 网络环境不同（4G 环境下预加载太多可能影响性能）

**建议**: 暂时固定为 5 个，后续可以基于用户行为数据分析优化（如根据点击率动态调整）。

### Q3: 是否需要支持批量预加载 API？

**背景**: 如果未来需要预加载更多报告（如 10-20 个），多次单独请求可能影响性能

**考虑**:
- 当前预加载 5 个报告已经足够
- 如果未来需要优化，可以新增批量预加载接口

**建议**: 暂不添加，等到有明确需求时再考虑。避免过度设计。