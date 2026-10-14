## Why

当前场景设置以抽屉（Drawer）方式打开，宽度仅 440px。当场景变量较多时，底部的 CSV 配置区域被挤压甚至无法加载，严重影响使用体验。需要将设置改为独立页面，提供充足的空间和更清晰的分区。

## What Changes

- 新增 `SceneSettingsPage.vue` 独立页面，包含左侧导航（基本信息 / 场景变量 / 数据源 CSV）和右侧内容区，采用双栏布局
- 新增路由 `/scenes/:id/settings`，从场景详情页的"设置"按钮点击后跳转至此页面
- **BREAKING**: 移除 `SceneDetailPage.vue` 中的设置抽屉（Drawer）及其相关状态，改为 `router.push` 跳转
- CSV 数据源卡片点击后展开内联表格编辑器，支持分页、行/列增删、单元格 inline 编辑
- 完全兼容当前深色/浅色主题配色系统（CSS 变量）

## Capabilities

### New Capabilities
- `scene-settings-page`: 场景设置独立页面，包含基本信息展示与编辑、场景变量管理、CSV 数据源管理与内联编辑

### Modified Capabilities
<!-- 无既有 spec 需要修改 -->

## Impact

- 前端路由：新增 `/scenes/:id/settings` 路由
- `SceneDetailPage.vue`：移除设置 Drawer，设置按钮改为路由跳转
- `SceneSettingsPage.vue`：新建组件，复用既有 API（场景详情、变量 CRUD、数据源 CRUD）
- 样式：使用既有 CSS 变量系统，不引入新设计 token
