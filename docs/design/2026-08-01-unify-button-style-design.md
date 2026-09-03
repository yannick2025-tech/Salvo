# 统一系统按钮风格设计

## 背景

系统当前存在两套主按钮样式，颜色不一致：

- `.btn-login-primary`：蓝色（`#58a6ff` / `#0969da`），用于登录、新建场景、启动测试等
- `.btn-primary`：青色（`var(--accent-primary)`），用于保存配置、添加节点等

同一个"保存"动作在不同页面呈现不同颜色，用户感知混乱。此外 `.btn-primary` 在 6 个 Vue 文件中各自重复定义，样式有细微差异。Toast 通知的 success 颜色也存在跨页面不一致。

## 目标

1. 全局主按钮统一为蓝色品牌色
2. 青色保留给图表、链接、节点状态等辅助场景
3. 消除 `.btn-primary` 的重复定义，统一到 global.css
4. Toast success 颜色跨页面统一

## 设计

### 新增 CSS 变量

```css
:root, [data-theme='dark'] {
  --btn-primary-bg: #58a6ff;
  --btn-primary-bg-hover: #388bfd;
}
[data-theme='light'] {
  --btn-primary-bg: #0969da;
  --btn-primary-bg-hover: #0860ca;
}
```

### 统一的按钮层级

| 类名 | 用途 | 背景 |
|------|------|------|
| `.btn-primary` | 主操作（登录/新建/启动/保存/上传/创建） | `var(--btn-primary-bg)` |
| `.btn-secondary` | 次要操作（取消/关闭） | 透明 + 边框 |
| `.btn-danger-confirm` | 危险操作（删除） | `var(--accent-danger)` |
| `.btn-outline` | 描边按钮（DAG 工具栏等辅助操作） | 青色边框 |
| `.btn-sm` | 小型按钮 | 透明 + 边框 |

### global.css 改动

- 新增 `--btn-primary-bg` / `--btn-primary-bg-hover` 变量
- 新增统一 `.btn-primary` 定义（含 hover、disabled）
- 删除 `.btn-login-primary` 定义

### 各 Vue 文件改动

1. **所有 `btn-login-primary` → `btn-primary`**（8 个文件）
2. **删除各文件内重复的 `.btn-primary` 局部定义**（6 个文件）
3. **LoginPage.vue**：`.btn-login` → `.btn-primary`，删除 `[data-theme='light'] .btn-login` 覆写

### Toast 统一

| 文件 | 改动 |
|------|------|
| SceneSettingsPage.vue | 删除 `[data-theme='dark'] .toast.success { background: #58a6ff; }` 覆写 |
| DagFlow.vue | `rgba(46,204,113,0.92)` → `var(--accent-success)` |
| RunnerPage.vue | 补充 `.toast.success { background: var(--accent-success); }` |

## 影响范围

- global.css
- LoginPage.vue
- ScenesPage.vue
- SceneDetailPage.vue
- SceneSettingsPage.vue
- RunnerPage.vue
- SettingsPage.vue
- UsersPage.vue
- PluginsPage.vue
- ReportDetailPage.vue
- DagFlow.vue

## 不改动

- 青色 `--accent-primary` 变量本身（图表、链接、节点状态继续使用）
- `.btn-outline`（青色描边，用于辅助操作）
- `.btn-danger-confirm`（红色危险按钮）
- `.btn-secondary`、`.btn-sm`（透明背景）
