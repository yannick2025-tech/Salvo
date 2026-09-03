# P9: Web UI + RBAC 设计文档

## 1. 概述

基于已确认的 Mock UI（`web/mock-ui.html`），开发功能完整的 Web UI，包含登录认证、RBAC 权限控制、实时仪表盘、场景管理、运行控制、报告查看等核心功能。

## 2. 技术栈

| 层级 | 技术 |
|------|------|
| 前端框架 | Vue 3 + Composition API + TypeScript |
| 构建工具 | Vite |
| UI 组件库 | Naive UI |
| 状态管理 | Pinia |
| 路由 | Vue Router 4 |
| 图表 | ECharts |
| DAG 编辑器 | @vue-flow/core（P9-B 阶段） |
| HTTP 客户端 | Axios |
| 实时通信 | WebSocket（P9-B 阶段） |
| 后端鉴权 | JWT (golang-jwt) |
| 密码加密 | bcrypt (golang.org/x/crypto) |

## 3. 架构

```
Browser (Vue 3 SPA)
  │ Axios + WebSocket
  ▼
Go API Server (:7789)
  │ JWT Auth Middleware → RBAC Middleware → Business Handlers
  ▼
SQLite (scenes, nodes, edges, users, roles, permissions, runs, traces...)
```

## 4. 后端新增模块

### 4.1 数据模型

**users 表**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | snowflake.ID | 主键 |
| email | string | 登录邮箱，唯一 |
| password_hash | string | bcrypt 哈希 |
| nickname | string | 显示名称 |
| role_id | snowflake.ID | 关联角色 |
| status | string | active / disabled |
| last_login_at | *time.Time | 最后登录时间 |
| created_at | time.Time | 创建时间 |
| updated_at | time.Time | 更新时间 |
| deleted_at | *time.Time | 软删除 |

**roles 表**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | snowflake.ID | 主键 |
| name | string | 角色名，唯一 |
| description | string | 描述 |
| is_builtin | bool | 内置角色不可删除 |
| created_at | time.Time | |
| updated_at | time.Time | |

**permissions 表**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | snowflake.ID | 主键 |
| resource | string | 资源名（scene, report, runner...） |
| action | string | 操作（read, write, run, export） |
| description | string | 描述 |
| created_at | time.Time | |

**role_permissions 表**

| 字段 | 类型 | 说明 |
|------|------|------|
| role_id | snowflake.ID | 角色 ID |
| permission_id | snowflake.ID | 权限 ID |

### 4.2 预置角色与权限

**内置角色**

| 角色 | 说明 |
|------|------|
| admin | 全部权限，含用户管理 |
| operator | 查看 + 运行操作，不能编辑场景和用户 |
| viewer | 全部菜单只读 |

**权限定义**

```
dashboard:read
scene:read / scene:write / scene:run
report:read / report:export
trace:read
runner:read / runner:write
user:read / user:write
role:read / role:write
settings:read / settings:write
```

**角色-权限映射**

| 权限 | admin | operator | viewer |
|------|:-----:|:--------:|:------:|
| dashboard:read | ✅ | ✅ | ✅ |
| scene:read | ✅ | ✅ | ✅ |
| scene:write | ✅ | ❌ | ❌ |
| scene:run | ✅ | ✅ | ❌ |
| report:read | ✅ | ✅ | ✅ |
| report:export | ✅ | ✅ | ❌ |
| trace:read | ✅ | ✅ | ✅ |
| runner:read | ✅ | ✅ | ✅ |
| runner:write | ✅ | ✅ | ❌ |
| user:read | ✅ | ❌ | ❌ |
| user:write | ✅ | ❌ | ❌ |
| role:read | ✅ | ❌ | ❌ |
| role:write | ✅ | ❌ | ❌ |
| settings:read | ✅ | ✅ | ✅ |
| settings:write | ✅ | ❌ | ❌ |

### 4.3 首次启动初始化

服务启动时检测 users 表是否为空，若为空则自动创建：
- admin 角色 + 全部权限
- operator 角色 + 运行权限
- viewer 角色 + 只读权限
- admin 用户（email: admin@salvo.local, password: admin）

### 4.4 新增 API 端点

**认证**

```
POST /api/v1/auth/login           # {email, password} → {token, user}
POST /api/v1/auth/logout          # 清除 token
POST /api/v1/auth/me              # 当前用户 + 权限列表
POST /api/v1/auth/change-password # {old_password, new_password}
```

**用户管理（admin）**

```
POST /api/v1/users/list
POST /api/v1/users/create
POST /api/v1/users/update
POST /api/v1/users/delete
```

**角色管理（admin）**

```
POST /api/v1/roles/list
POST /api/v1/roles/create
POST /api/v1/roles/update
POST /api/v1/roles/delete
```

### 4.5 JWT 鉴权流程

1. 用户 POST /api/v1/auth/login → 验证邮箱密码 → 签发 JWT
2. JWT payload: `{user_id, role_id, exp}`
3. JWT secret 从配置文件读取，默认随机生成
4. Token 有效期 24 小时
5. 前端每次请求携带 `Authorization: Bearer <token>`
6. JWT 中间件解析 token，注入 user_id/role_id 到 request context
7. RBAC 中间件根据路由映射的 resource:action 检查权限
8. /api/v1/auth/* 路由不需要鉴权

### 4.6 Go 依赖新增

```
github.com/golang-jwt/jwt/v5
golang.org/x/crypto/bcrypt
```

## 5. 前端架构

### 5.1 项目结构

```
web/
├── index.html
├── vite.config.ts
├── package.json
├── tsconfig.json
├── env.d.ts
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── router/
│   │   └── index.ts
│   ├── stores/
│   │   ├── auth.ts
│   │   ├── scene.ts
│   │   ├── runner.ts
│   │   └── settings.ts
│   ├── api/
│   │   ├── client.ts
│   │   ├── auth.ts
│   │   ├── scene.ts
│   │   ├── runner.ts
│   │   ├── report.ts
│   │   └── trace.ts
│   ├── composables/
│   │   ├── usePermission.ts
│   │   ├── useChart.ts
│   │   └── useTheme.ts
│   ├── layouts/
│   │   └── MainLayout.vue
│   ├── views/
│   │   ├── Login.vue
│   │   ├── Dashboard.vue
│   │   ├── Scenes.vue
│   │   ├── Runner.vue
│   │   ├── Traces.vue
│   │   ├── Reports.vue
│   │   ├── Settings.vue
│   │   └── UserManagement.vue
│   ├── components/
│   │   ├── charts/
│   │   ├── metrics/
│   │   └── common/
│   ├── styles/
│   │   ├── variables.css
│   │   └── global.css
│   └── types/
│       └── index.ts
```

### 5.2 路由与权限守卫

```typescript
const routes = [
  { path: '/login', component: Login, meta: { public: true } },
  { path: '/', component: MainLayout, meta: { requiresAuth: true }, children: [
    { path: '', redirect: '/dashboard' },
    { path: 'dashboard', component: Dashboard, meta: { permission: 'dashboard:read' } },
    { path: 'scenes', component: Scenes, meta: { permission: 'scene:read' } },
    { path: 'runner', component: Runner, meta: { permission: 'runner:read' } },
    { path: 'traces', component: Traces, meta: { permission: 'trace:read' } },
    { path: 'reports', component: Reports, meta: { permission: 'report:read' } },
    { path: 'settings', component: Settings, meta: { permission: 'settings:read' } },
    { path: 'users', component: UserManagement, meta: { permission: 'user:read' } },
  ]},
]

router.beforeEach((to, from, next) => {
  if (to.meta.public) return next()
  if (!authStore.token) return next('/login')
  if (to.meta.permission && !authStore.hasPermission(to.meta.permission)) {
    return next('/403')
  }
  next()
})
```

### 5.3 主题系统

沿用 Mock UI 的深色/浅色双主题设计：
- Naive UI `n-config-provider` 管理组件主题
- CSS 变量管理自定义样式主题
- localStorage 持久化主题选择

### 5.4 图表系统

基于 ECharts 封装 `useChart` composable：
- 支持柱状图/折线图切换
- 支持时间范围选择（1m/5m/15m/1h/6h/24h）
- 支持点击放大（模态框全宽展示）
- 自动适配深色/浅色主题

## 6. 开发分期

### P9-A（当前阶段）

- 后端：User/Role/Permission 模型 + 数据库迁移
- 后端：JWT 鉴权 + RBAC 中间件
- 后端：Auth/Users/Roles API 端点
- 前端：Vue 3 + Vite 项目脚手架
- 前端：Login 页面 + Auth 流程
- 前端：MainLayout + 主题系统 + 路由守卫
- 前端：Dashboard（含 Node 级别指标 + 图表交互）
- 前端：Scenes（列表 + 基础 CRUD）
- 前端：Runner（运行控制 + 实时指标）
- 前端：Reports（报告查看 + Node 级别指标）
- 前端：Traces（Trace 列表 + 详情）
- 前端：Settings（系统配置）
- 前端：UserManagement（用户/角色管理）

### P9-B（后续迭代）

- DAG 可视化编辑器（@vue-flow/core）
- WebSocket 实时指标推送
- 报告导出（HTML/PDF）
- 场景变量/插件配置 UI
