## 1. 路由与页面骨架

- [x] 1.1 在 `router/index.ts` 中新增路由 `/scenes/:id/settings`，引入 `SceneSettingsPage` 组件
- [x] 1.2 创建 `SceneSettingsPage.vue` 文件，搭建双栏布局骨架（左导航 + 右内容区）
- [x] 1.3 实现页面头部（返回按钮 + 标题 + 场景名 tag）

## 2. 左侧导航与 Tab 切换

- [x] 2.1 实现左侧导航项（基本信息/场景变量/数据源 CSV），含 SVG 图标
- [x] 2.2 实现导航项 active 状态高亮（左边框 + accent-primary 文字色）
- [x] 2.3 实现 Tab 切换逻辑，右侧内容区跟随切换

## 3. 基本信息区

- [x] 3.1 实现扁平表格布局（两列 label-value 横排，border 分隔）
- [x] 3.2 展示场景属性（状态 badge、创建时间、描述、默认超时 input、DAG 节点数、变量数）

## 4. 场景变量区

- [x] 4.1 实现变量表格（表头 + 变量行：变量名 input + = + 值 input + 删除按钮）
- [x] 4.2 实现"+ 添加变量"按钮（dashed border 样式）
- [x] 4.3 对接变量 CRUD API（addVariable / deleteVariable）

## 5. 数据源 (CSV) 区

- [x] 5.1 实现"上传 CSV"按钮和数据源卡片列表（图标 + 文件名 + 列·行数 + 删除）
- [x] 5.2 实现 CSV 内联表格编辑器（展开/收起、表头列名编辑、单元格 inline 编辑）
- [x] 5.3 实现编辑器底栏（分页、行列增删、保存按钮）
- [x] 5.4 对接数据源 CRUD API

## 6. Drawer 移除与跳转改造

- [x] 6.1 从 `SceneDetailPage.vue` 中移除设置 Drawer 及 `showSettings` ref
- [x] 6.2 将设置按钮的 click handler 改为 `router.push('/scenes/' + id + '/settings')`

## 7. 主题兼容与验收

- [x] 7.1 验证深色主题下所有元素配色正确
- [x] 7.2 验证浅色主题下所有元素配色正确
- [x] 7.3 验证变量多（8+）时 CSV 区域可正常访问和展示
