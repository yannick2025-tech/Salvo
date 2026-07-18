## ADDED Requirements

### Requirement: Scene settings page route
系统 SHALL 提供路由 `/scenes/:id/settings`，渲染 `SceneSettingsPage` 组件。

#### Scenario: Navigate to settings page
- **WHEN** 用户在场景详情页点击"设置"按钮
- **THEN** 系统导航至 `/scenes/:id/settings`，加载场景设置页面

#### Scenario: Direct URL access
- **WHEN** 用户直接访问 `/scenes/123/settings`
- **THEN** 系统加载 ID 为 123 的场景设置页面

### Requirement: Settings page layout
设置页面 SHALL 采用双栏布局：左侧导航（200px）+ 右侧内容区。

#### Scenario: Left navigation tabs
- **WHEN** 页面加载完成
- **THEN** 左侧导航显示三个选项：基本信息（齿轮图标）、场景变量（花括号图标）、数据源 CSV（表格图标），默认选中"基本信息"

#### Scenario: Tab switching
- **WHEN** 用户点击左侧导航项
- **THEN** 右侧内容区切换为对应分区内容，选中项高亮（accent-primary 左边框 + 文字色）

### Requirement: Basic info section
基本信息区 SHALL 以扁平表格形式展示场景属性（状态、创建时间、描述、默认超时、DAG 节点数、变量数），两列布局，label 左对齐、value 右对齐。

#### Scenario: Display basic info
- **WHEN** 用户选中"基本信息"导航项
- **THEN** 右侧显示场景基本信息表格，状态显示为 badge，默认超时为可编辑 input

### Requirement: Variables section
场景变量区 SHALL 以表格形式展示变量列表（变量名 / = / 值 / 删除），支持添加和删除变量。

#### Scenario: Display variables
- **WHEN** 用户选中"场景变量"导航项
- **THEN** 右侧显示变量表格，每行为：变量名 input + = + 值 input + 删除按钮

#### Scenario: Add variable
- **WHEN** 用户点击"+ 添加变量"按钮
- **THEN** 表格底部新增一行空变量

#### Scenario: Delete variable
- **WHEN** 用户点击某行的删除按钮
- **THEN** 该变量行被移除

### Requirement: CSV datasource section
数据源区 SHALL 展示已上传的 CSV 文件卡片列表，支持上传和删除。点击卡片展开内联表格编辑器。

#### Scenario: Display datasource list
- **WHEN** 用户选中"数据源 (CSV)"导航项
- **THEN** 右侧显示上传按钮和 CSV 文件卡片列表，每张卡片显示文件名、列数·行数

#### Scenario: Open CSV editor
- **WHEN** 用户点击某张数据源卡片
- **THEN** 卡片下方展开内联表格编辑器，显示该 CSV 的数据，支持 inline 编辑、分页、行列增删

#### Scenario: Close CSV editor
- **WHEN** 用户点击编辑器头部的关闭按钮
- **THEN** CSV 编辑器收起

### Requirement: Back navigation
设置页面头部 SHALL 提供"返回场景"按钮，点击后返回场景详情页。

#### Scenario: Click back button
- **WHEN** 用户点击"返回场景"按钮
- **THEN** 系统导航回 `/scenes/:id`

### Requirement: Drawer removal
SceneDetailPage 中的设置 Drawer SHALL 被移除，设置按钮改为路由跳转。

#### Scenario: Settings button navigation
- **WHEN** 用户在场景详情页点击设置图标
- **THEN** 系统导航至 `/scenes/:id/settings`，而非打开 Drawer

### Requirement: Theme compatibility
设置页面 SHALL 完全兼容深色和浅色主题，使用项目 CSS 变量系统。

#### Scenario: Dark theme rendering
- **WHEN** 系统处于深色主题
- **THEN** 设置页面所有元素使用深色主题配色

#### Scenario: Light theme rendering
- **WHEN** 系统处于浅色主题
- **THEN** 设置页面所有元素使用浅色主题配色
