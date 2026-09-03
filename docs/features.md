# Salvo 功能清单

## 平台简介

Salvo 是一个**配置驱动的通用性能测试平台**，专为 API 和微服务架构的性能测试而设计。

### 核心特性

- **DAG 请求流编排**：支持可视化编排复杂的请求链路，包括 HTTP 请求、延迟、条件分支、循环等节点类型
- **多场景并发测试**：支持同时运行多个测试场景，按时间或按次数两种运行模式
- **实时监控与追踪**：WebSocket 实时推送执行进度，支持聚合视图和单链路视图切换，4 层 Trace 追踪（Run → Chain → Node → Span）
- **丰富的测试报告**：自动生成包含总体指标、节点详情、失败请求分析、图表可视化的 HTML 报告

### 扩展性架构

#### 插件系统

Salvo 采用插件化架构，支持两种插件类型：

- **Go Plugin（SO 插件）**：已实现，通过动态链接库（.so 文件）热加载，支持加密解密、自定义协议处理等业务定制功能
- **Lua Plugin**：计划中，将支持 Lua 脚本插件，用于限速、流量控制等场景

#### 协议扩展性

平台预留了协议抽象接口，当前支持 HTTP/HTTPS 协议，可方便地扩展到其他协议：

- **DATABASE**：MySQL、PostgreSQL、Redis 等数据库协议
- **FTP/SFTP**：文件传输协议
- **gRPC**：RPC 框架协议
- **WebSocket**：长连接协议
- 其他自定义协议

#### 业务扩展性

Salvo 最大的优势在于**零代码业务适配**：

- **YAML 配置驱动**：只需编写 YAML 配置文件即可定义完整的测试场景，包括请求链路、变量参数、数据源、断言逻辑等
- **内置生成器**：提供 UUID、邮箱、日期、随机数、枚举等 13+ 种数据生成器，满足各种测试数据需求
- **数据源支持**：支持 CSV 数据源导入，实现参数化测试
- **变量与提取**：支持场景级变量、响应数据提取、变量传递，轻松处理复杂的业务关联场景
- **加密解密**：内置通用加密插件，支持 Manhattan 等业务定制加密方案

通过 YAML 配置 + 插件扩展，Salvo 可以快速适应不同的业务系统测试需求，无需修改核心代码。

---

## 前端功能

| 一级菜单 | 二级功能 | 页面路径 | 功能描述 | 已实现 |
|---------|---------|---------|---------|:------:|
| 仪表盘 | 场景选择与时间范围 | /dashboard | 选择场景，显示时间范围、持续时长、实时指示器，支持刷新频率设置 | ✓ |
| | 概览指标卡片 | | 总请求数、成功率、平均延迟、P99、QPS 等汇总指标 | ✓ |
| | QPS 趋势图 | | 吞吐量趋势，支持平滑/阶梯切换，支持时间范围框选 | ✓ |
| | 延迟分布图 | | P50/P90/P95/P99 延迟百分位带，支持端到端/纯 HTTP 切换 | ✓ |
| | 错误率趋势图 | | 错误率趋势，支持平滑/阶梯切换 | ✓ |
| | 最近运行列表 | | 展示最近运行记录及 QPS/P99/成功率 | ✓ |
| | 实时进度（聚合视图） | | 按节点聚合展示通过/失败/跳过/运行中数量，WebSocket 实时推送 | ✓ |
| | 实时进度（单链路视图） | | 选择具体链路查看循环进度和节点状态 | ✓ |
| 场景管理 | 场景列表 | /scenes | 查看所有场景，显示状态、创建时间、测试时间、持续时间 | ✓ |
| | 新建场景 | | 弹窗输入名称和描述创建场景 | ✓ |
| | 导入 YAML | | 通过 YAML 文件导入场景配置 | ✓ |
| | 编辑场景 | | 跳转至场景详情页编辑 | ✓ |
| | 实时进度 | | 跳转至场景详情页查看执行进度 | ✓ |
| | 删除场景 | | 确认后删除场景 | ✓ |
| | 场景详情/DAG 编辑 | /scenes/:id | DAG 请求流画布，支持可视化编辑节点和边 | ✓ |
| | 节点类型 | | 支持 HTTP 请求、延迟、条件分支、循环（while）、定时触发等节点 | ✓ |
| | 节点配置 | | 配置请求参数、重试策略、错误阻断、变量提取等 | ✓ |
| | 场景设置 | /scenes/:id/settings | 配置场景变量、数据源等 | ✓ |
| | 变量管理 | | 场景级变量设置和批量设置 | ✓ |
| | 数据源管理 | | CSV 数据源上传、预览、管理 | ✓ |
| 运行控制 | 启动场景 | /runner | 配置并发数、持续时间、运行模式（按时间/按次数）、总请求数 | ✓ |
| | 运行状态监控 | | 查看运行中场景的实时指标（工作线程、总请求、成功/失败、P99） | ✓ |
| | 停止场景 | | 确认后停止运行中的场景 | ✓ |
| | 实时进度 | | 跳转至场景详情页查看执行进度 | ✓ |
| 测试报告 | 报告列表 | /reports | 查看历史测试报告，显示场景、状态、请求数、成功率、延迟指标 | ✓ |
| | 导出报告 | | 导出单个 HTML 报告 | ✓ |
| | 批量导出 | | 批量导出多个报告 | ✓ |
| | 报告详情 | /reports/:id | 查看详细的测试报告 | ✓ |
| | 报告内容块 | | <ul><li>总体指标（总请求、成功率、延迟、吞吐量）</li><li>各节点运行时指标</li><li>失败节点详情（含请求 Method/URL/Headers/Body）</li><li>失败请求响应详情（Status/Headers/Body）</li><li>QPS/延迟/错误率图表</li></ul> | ✓ |
| 链路追踪 | Trace 列表 | /traces | 查看请求链路追踪列表，支持分页 | ✓ |
| | Trace 详情 | /traces/:id | 查看 TraceID/RunID/场景/状态/耗时，展示 Span 列表 | ✓ |
| | 4 层追踪信息 | | <ul><li>Run 层 - 运行记录</li><li>Chain 层 - 链路实例</li><li>Node 层 - 节点执行</li><li>Span 层 - 请求跨度</li></ul> | ✓ |
| 用户管理 | 用户列表 | /users | 管理系统用户（需 users:read 权限） | ✓ |
| | 创建用户 | | 创建新用户，分配角色 | ✓ |
| | 编辑用户 | | 修改用户信息和角色 | ✓ |
| | 删除用户 | | 删除用户 | ✓ |
| SO 插件管理 | 插件列表 | /plugins | 查看 SO 插件列表（需 plugins:read 权限） | ✓ |
| | 上传插件 | | 上传 .so 文件创建插件 | ✓ |
| | 插件详情 | | 查看插件配置和状态 | ✓ |
| | 启用/禁用 | | 切换插件状态，启用时热加载 | ✓ |
| | 插件配置 | | 更新插件配置参数 | ✓ |
| | 删除插件 | | 删除插件记录和文件 | ✓ |
| 个人设置 | 设置页 | /settings | 修改个人信息、密码等 | ✓ |
| 登录 | 登录页 | /login | 邮箱/密码登录，支持跳转回原页面 | ✓ |

---

## 后端功能

| 模块 | 子模块 | 功能 | API 端点 | 已实现 |
|-----|-------|------|---------|:------:|
| 认证模块 | - | 登录 | POST /api/v1/auth/login | ✓ |
| | | 登出 | POST /api/v1/auth/logout | ✓ |
| | | 获取当前用户 | POST /api/v1/auth/me | ✓ |
| | | 修改密码 | POST /api/v1/auth/change-password | ✓ |
| | | 重置密码 | POST /api/v1/auth/reset-password | ✓ |
| RBAC | 用户管理 | 用户 CRUD | /api/v1/users/* | ✓ |
| | 角色管理 | 角色 CRUD、权限分配 | /api/v1/roles/* | ✓ |
| | 权限控制 | 路由级权限校验（resource:action） | handleAuth 中间件 | ✓ |
| | 权限资源 | <ul><li>scene:read/write - 场景读写</li><li>runner:read - 运行控制</li><li>admin:write - 管理员权限</li><li>users:read - 用户管理</li><li>plugins:read - 插件管理</li></ul> | | ✓ |
| 仪表盘 | - | 概览数据 | POST /api/v1/dashboard/overview | ✓ |
| | | 历史趋势数据 | POST /api/v1/dashboard/history | ✓ |
| 场景管理 | 场景 CRUD | 列表、创建、获取、更新、删除 | /api/v1/scenes/list,create,get,update,delete | ✓ |
| | 导入导出 | YAML 导入导出 | /api/v1/scenes/import,export | ✓ |
| | 节点管理 | DAG 节点增删改查 | /api/v1/scenes/nodes/* | ✓ |
| | 边管理 | DAG 边增删查 | /api/v1/scenes/edges/* | ✓ |
| | 变量管理 | 场景变量设置和批量设置 | /api/v1/scenes/variables/* | ✓ |
| | 数据源管理 | CSV 数据源上传、预览、管理 | /api/v1/scenes/datasources/* | ✓ |
| | 运行控制 | 启动、停止、查询状态 | /api/v1/scenes/start,stop,status | ✓ |
| 插件系统 | 内置插件 | 内置插件列表和配置管理 | /api/v1/plugins/list,config | ✓ |
| | Go Plugin | SO 插件上传、创建、列表、获取、状态更新、配置、删除 | /api/v1/so-plugins/* | ✓ |
| | Go Plugin 热加载 | 启用时动态加载 .so 文件 | SO Loader | ✓ |
| | Lua Plugin | Lua 脚本插件（限速等） | - | ✗ |
| 加密解密 | 通用加密 | AES-CBC/GCM 加密/解密插件 | 内置 crypto 插件 | ✓ |
| | 业务定制加密 | Manhattan 等定制加密（IV = key[:16]、Bcrypt支持定制化Salt等） | SO 插件方式 | ✓ |
| 生成器 | 基础类型 | <ul><li>uuid - UUID 生成</li><li>email - 邮箱生成</li><li>date - 日期生成</li><li>date-time - 日期时间生成</li><li>random-string - 随机字符串</li><li>enum-string - 枚举字符串</li><li>random-int - 随机整数</li><li>increment-int - 自增整数</li><li>random-float - 随机浮点数</li><li>random-bool - 随机布尔</li><li>weighted-bool - 加权布尔</li><li>null - 空值</li><li>array - 数组生成</li></ul> | POST /api/v1/generators/list | ✓ |
| 测试报告 | 报告查询 | 报告列表、详情 | /api/v1/reports/list,get | ✓ |
| | 报告导出 | 单个 HTML 报告导出 | GET /api/v1/reports/{id}/export | ✓ |
| | 批量导出 | 批量导出多个报告 | POST /api/v1/reports/batch-export | ✓ |
| | 报告内容 | <ul><li>总体指标（总请求、成功率、延迟、吞吐量）</li><li>各节点运行时指标</li><li>失败节点详情（含请求 Method/URL/Headers/Body）</li><li>失败请求响应详情（Status/Headers/Body）</li><li>QPS/延迟/错误率图表</li></ul> | | ✓ |
| 运行记录 | - | 运行记录列表 | POST /api/v1/runs/list | ✓ |
| | | 运行记录详情 | POST /api/v1/runs/get | ✓ |
| 链路追踪 | 4 层追踪 | <ul><li>Run 层 - 运行记录</li><li>Chain 层 - 链路实例</li><li>Node 层 - 节点执行</li><li>Span 层 - 请求跨度</li></ul> | /api/v1/traces/* | ✓ |
| | Trace 查询 | 列表、详情、按运行查询 | /api/v1/traces/list,get,get-by-run | ✓ |
| WebSocket | 实时推送 | 运行状态、节点进度、循环索引推送 | GET /ws | ✓ |
| | 消息去重 | 按 (run_id, chain_id, node_id) 去重 | | ✓ |

---

## 附录：节点类型说明

| 节点类型 | 说明 |
|---------|------|
| HTTP 请求 | 发送 HTTP 请求，支持变量替换、响应提取 |
| 延迟 | 等待指定时间（think_time） |
| 条件分支 | 根据条件表达式选择执行路径 |
| 循环（while） | 循环执行子节点，支持最大迭代次数和失败行为配置 |
| 定时触发 | 按时间间隔触发节点执行 |

---

## 附录：运行模式说明

| 模式 | 说明 |
|-----|------|
| 按时间 | 持续运行指定时长（秒） |
| 按次数 | 发送指定数量的请求后停止 |
