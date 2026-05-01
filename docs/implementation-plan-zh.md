# Salvo 实施计划

**日期：** 2026-05-01  
**状态：** 进行中  
**仓库：** https://github.com/yannick2025-tech/Salvo

---

## 阶段总览

| 阶段 | 模块 | 依赖 | 状态 |
|------|------|------|------|
| P0 | 项目骨架 + 基础设施 | 无 | 待开始 |
| P1 | 核心引擎 | P0 | 待开始 |
| P2 | 协议层 | P1 | 待开始 |
| P2.5 | 模拟 HTTP 服务器 | P2 | 待开始 |
| P3 | 插件系统 | P2 | 待开始 |
| P4 | 参数生成器 | P3 | 待开始 |
| P5 | 数据存储 | P0 | 待开始 |
| P6 | REST 接口 | P1+P5 | 待开始 |
| P7 | 追踪系统 | P0+P1 | 待开始 |
| P8 | 场景运行器 | P1-P4 | 待开始 |
| P9 | 网页界面 | P6 | 待开始 |
| P10 | 集成与发布 | 全部 | 待开始 |

---

## P0：项目骨架 + 基础设施

**目标：** 目录结构、日志、雪花 ID、配置加载

### 任务清单

- [ ] 按设计规格创建项目目录结构
- [ ] 实现雪花 ID 生成器（`internal/pkg/snowflake/`）
  - [ ] 测试：跨协程生成唯一 ID
  - [ ] 测试：JSON 序列化/反序列化为字符串
  - [ ] 实现生成器
- [ ] 实现结构化日志（`internal/logger/`）
  - [ ] 定义 `Logger` 接口
  - [ ] 测试：文本格式输出
  - [ ] 测试：JSON 格式输出
  - [ ] 测试：追踪标识注入
  - [ ] 实现基于 Zap 的日志
- [ ] 实现配置加载器（`internal/config/`）
  - [ ] 定义与 YAML 模式匹配的 `Config` 结构体
  - [ ] 测试：从 YAML 文件加载
  - [ ] 测试：默认值
  - [ ] 实现 YAML 加载器
- [ ] 创建入口（`cmd/salvo/main.go`）
- [ ] 里程碑提交：`feat(infra): project skeleton with logger, snowflake, config`

### 验收标准

- `go build ./...` 成功
- `go test ./...` 通过且覆盖率 ≥ 80%
- 日志同时输出文本和 JSON 格式
- 雪花 ID 在 JSON 中序列化为字符串

---

## P1：核心引擎

**目标：** DAG 执行器、协程池、变量系统、生命周期、定时器

### P1.1：DAG 执行器（`internal/core/dag/`）

- [ ] 定义 `Node`、`Edge`、`DAG` 接口
- [ ] 测试：添加节点和边
- [ ] 测试：检测环（验证失败）
- [ ] 测试：拓扑排序顺序
- [ ] 测试：根节点识别
- [ ] 实现 `dag` 包
- [ ] 里程碑提交：`type(dag): define DAG interfaces`

- [ ] 测试：同步执行（等待响应）
- [ ] 测试：异步执行（触发后不等待）
- [ ] 测试：条件边（表达式分支）
- [ ] 测试：节点循环次数
- [ ] 测试：扇出（并行子节点）
- [ ] 测试：扇入（等待所有父节点）
- [ ] 实现 DAG 执行器
- [ ] 里程碑提交：`feat(dag): implement DAG executor`

### P1.2：协程池（`internal/core/pool/`）

- [ ] 定义 `Pool`、`Task`、`RunMode` 接口
- [ ] 测试：向固定池提交任务
- [ ] 测试：池大小限制生效
- [ ] 测试：固定时长模式（运行 X 时间后停止）
- [ ] 测试：固定次数模式（运行 N 次后停止）
- [ ] 测试：通过 context 优雅关闭
- [ ] 测试：提交并等待结果
- [ ] 实现协程池
- [ ] 里程碑提交：`feat(pool): implement goroutine pool`

### P1.3：变量系统（`internal/core/variable/`）

- [ ] 定义 `Scope`、`Store` 接口
- [ ] 测试：各作用域的设置/获取
- [ ] 测试：查找顺序（接口 → 场景 → 全局）
- [ ] 测试：内层作用域覆盖外层
- [ ] 测试：解析 `${A.token}` 表达式
- [ ] 测试：解析 map 中的所有参数
- [ ] 实现变量存储
- [ ] 里程碑提交：`feat(variable): implement variable system`

### P1.4：生命周期（`internal/core/lifecycle/`）

- [ ] 定义生命周期钩子接口
- [ ] 测试：全局初始化/清理只调用一次
- [ ] 测试：场景初始化/清理每个场景调用一次
- [ ] 测试：初始化可以设置变量
- [ ] 测试：出错时清理仍然执行
- [ ] 实现生命周期管理器
- [ ] 里程碑提交：`feat(lifecycle): implement lifecycle management`

### P1.5：定时器（`internal/core/timer/`）

- [ ] 定义 `TimerType`、`Timer` 接口
- [ ] 测试：周期定时器每 X 秒触发
- [ ] 测试：思考时间等待 X 秒后触发
- [ ] 测试：随机延迟在 [最小值, 最大值] 区间
- [ ] 测试：定时器尊重 context 取消
- [ ] 实现定时器
- [ ] 里程碑提交：`feat(timer): implement ticker, thinktime, random timers`

### P1.6：Context 级联（`internal/core/contextx/`）

- [ ] 测试：全局取消取消所有场景
- [ ] 测试：场景超时不影响全局
- [ ] 测试：节点超时不影响场景
- [ ] 测试：父取消传播到子
- [ ] 实现 context 级联辅助函数
- [ ] 里程碑提交：`feat(contextx): implement context timeout cascade`

### 验收标准

- 所有核心引擎测试通过且覆盖率 ≥ 80%
- DAG 可表达同步/异步/条件/循环/扇出/扇入
- 协程池强制固定大小并支持两种运行模式
- 变量在三个作用域间正确解析

---

## P2：协议层

**目标：** Protocol 接口 + HTTP 实现

### 任务清单

- [ ] 定义 `Protocol`、`Request`、`Response` 接口（`internal/protocol/`）
- [ ] 测试：接口合规性
- [ ] 里程碑提交：`type(protocol): define Protocol interface`

- [ ] 实现 HTTP 协议（`internal/protocol/http/`）
- [ ] 测试：GET 请求
- [ ] 测试：带 JSON 体的 POST 请求
- [ ] 测试：带请求头的请求
- [ ] 测试：通过 context 超时
- [ ] 测试：响应解析（状态码、头、体、延迟）
- [ ] 实现 HTTP 协议
- [ ] 里程碑提交：`feat(protocol): implement HTTP protocol`

### 验收标准

- HTTP 协议通过所有测试
- Protocol 接口足够通用，未来可扩展 DB/FTP/gRPC

---

## P2.5：模拟 HTTP 服务器

**目标：** 内置测试服务器用于端到端测试

### 任务清单

- [ ] 创建 `test/mockserver/` 包
- [ ] 实现 `/api/login`（认证 + 令牌）
- [ ] 实现 `/api/users` 增删改查（创建、获取、列表、更新、删除）
- [ ] 实现 `/api/orders`（创建、列表）
- [ ] 实现 `/api/upload`（文件上传）
- [ ] 实现 `/api/delay/:ms`（可配置延迟）
- [ ] 实现 `/api/status/:code`（任意状态码）
- [ ] 实现 `/api/echo`（回显请求体）
- [ ] 实现 `/api/headers`（回显请求头）
- [ ] 实现 `/api/encrypt`（加密请求体往返）
- [ ] 实现 `/api/chunked`（分块传输）
- [ ] 实现 `/api/redirect/:count`（链式重定向）
- [ ] 实现 `/api/error`（随机 500/502/503）
- [ ] 添加可配置延迟和错误率
- [ ] 添加 CORS 支持
- [ ] 测试：每个接口正确响应
- [ ] 测试：可配置延迟生效
- [ ] 测试：错误率生效
- [ ] 里程碑提交：`feat(mockserver): implement mock HTTP server for E2E testing`

### 验收标准

- 全部 18 个接口可用
- 可通过 `go test` 或独立二进制启动
- 延迟和错误率可配置

---

## P3：插件系统

**目标：** 插件注册中心、内置插件、Lua 引擎

### P3.1：插件注册中心（`internal/plugin/`）

- [ ] 定义 `Plugin`、`HookPoint`、`Registry` 接口
- [ ] 测试：注册插件到钩子
- [ ] 测试：按钩子点获取插件
- [ ] 测试：插件生命周期（初始化 → 请求前 → 响应后 → 关闭）
- [ ] 实现注册中心
- [ ] 里程碑提交：`type(plugin): define Plugin and Registry interfaces`

### P3.2：限速插件（`internal/plugin/ratelimit/`）

- [ ] 测试：全局每秒请求数限制生效
- [ ] 测试：单 URL 每秒请求数限制生效
- [ ] 测试：令牌桶允许突发
- [ ] 实现限速器
- [ ] 里程碑提交：`feat(ratelimit): implement rate limiter plugin`

### P3.3：加解密插件（`internal/plugin/crypto/`）

- [ ] 测试：MD5 哈希
- [ ] 测试：SHA256 哈希
- [ ] 测试：AES 加解密
- [ ] 测试：DES 加解密
- [ ] 测试：RSA 加解密
- [ ] 测试：DSA 签名/验证
- [ ] 测试：BCRYPT 哈希/验证
- [ ] 实现所有算法
- [ ] 里程碑提交：`feat(crypto): implement crypto plugin with MD5/SHA256/AES/DES/RSA/DSA/BCRYPT`

### P3.4：报告插件（`internal/plugin/report/`）

- [ ] 测试：HTML 报告生成
- [ ] 测试：Prometheus 指标端点
- [ ] 测试：实时指标推送
- [ ] 实现报告插件
- [ ] 里程碑提交：`feat(report): implement HTML and Prometheus report plugins`

### P3.5：Lua 引擎（`internal/plugin/lua/`）

- [ ] 测试：加载并执行 Lua 脚本
- [ ] 测试：Lua 插件可读写变量
- [ ] 测试：Lua 插件可发起 HTTP 请求
- [ ] 测试：Lua 插件可写日志
- [ ] 测试：Lua 插件覆盖内置加密
- [ ] 使用 GopherLua 实现 Lua 引擎
- [ ] 里程碑提交：`feat(lua): implement Lua custom plugin engine`

### 验收标准

- 所有内置插件通过测试
- Lua 插件可访问变量、日志、HTTP
- 插件注册中心正确路由钩子

---

## P4：参数生成器

**目标：** JSON Schema 解析、内置生成器、自定义 Lua 生成器

### 任务清单

- [ ] 定义 `Schema`、`Generator` 接口（`internal/generator/`）
- [ ] 实现 JSON Schema Draft 7 解析器（`internal/generator/schema/`）
- [ ] 测试：解析 Swagger/OpenAPI 规范
- [ ] 测试：解析 JSON Schema Draft 7
- [ ] 里程碑提交：`type(generator): define Schema and Generator interfaces`

- [ ] 实现内置生成器（`internal/generator/builtin/`）
- [ ] 测试：RandomString、RegexString、EnumString
- [ ] 测试：UUIDGenerator、EmailGenerator、DateGenerator
- [ ] 测试：RandomFloat、RandomInt、IncrementInt
- [ ] 测试：RandomBool、WeightedBool
- [ ] 测试：ArrayGenerator、ObjectGenerator
- [ ] 里程碑提交：`feat(generator): implement built-in parameter generators`

- [ ] 实现自定义 Lua 生成器
- [ ] 测试：Lua 生成器产生值
- [ ] 测试：Lua 生成器与内置生成器并列注册
- [ ] 里程碑提交：`feat(generator): implement custom Lua generators`

### 验收标准

- 支持所有 JSON Schema Draft 7 关键字
- 生成器产生有效的、符合模式的值
- Lua 生成器无缝集成

---

## P5：数据存储

**目标：** ent 对象关系映射、数据模型、Repository、数据库迁移

### 任务清单

- [ ] 使用 MySQL/PostgreSQL/SQLite 方言初始化 ent
- [ ] 定义模式：Scene、Node、Edge、Variable、PluginConfig、Report、RunRecord
- [ ] 生成 ent 代码
- [ ] 实现带雪花 ID + 软删除的 `Model` 基础结构
- [ ] 测试：创建并检索实体
- [ ] 测试：软删除过滤掉已删除记录
- [ ] 测试：雪花 ID 的 JSON 序列化为字符串
- [ ] 实现 Repository 接口（`internal/store/repo/`）
- [ ] 测试：通过 Repository 进行增删改查
- [ ] 实现数据库迁移（`internal/store/migration/`）
- [ ] 测试：迁移升级和回滚
- [ ] 里程碑提交：`feat(store): implement data storage with ent ORM`

### 验收标准

- 所有模型使用雪花 ID 并以字符串序列化 JSON
- 软删除在所有 Repository 中生效
- 迁移支持 MySQL、PostgreSQL、SQLite

---

## P6：REST 接口

**目标：** 全 POST 接口层、中间件、数据传输对象

### 任务清单

- [ ] 定义所有接口的数据传输对象（`internal/api/dto/`）
- [ ] 实现场景管理处理器（`internal/api/handler/`）
- [ ] 实现场景执行处理器
- [ ] 实现 DAG 节点/边处理器
- [ ] 实现报告处理器
- [ ] 实现插件处理器
- [ ] 实现变量处理器
- [ ] 实现 WebSocket 实时指标
- [ ] 使用 httptest 测试每个接口
- [ ] 实现中间件（日志、恢复、CORS）
- [ ] 使用 chi/echo 路由器绑定路由
- [ ] 里程碑提交：`feat(api): implement REST API layer with all POST endpoints`

### 验收标准

- 所有接口使用 POST 方法
- 请求/响应数据传输对象已验证
- WebSocket 流式推送实时指标

---

## P7：追踪系统

**目标：** 四级追踪、追踪标识传播

### 任务清单

- [ ] 定义 `Span`、`Tracer` 接口（`internal/trace/`）
- [ ] 测试：开始/结束 span
- [ ] 测试：父子关系
- [ ] 测试：从 context 获取 span
- [ ] 测试：向日志注入追踪标识
- [ ] 测试：四级传播（场景 → 链路 → 接口 → 函数）
- [ ] 实现追踪器
- [ ] 与日志集成，自动注入追踪标识
- [ ] 里程碑提交：`feat(trace): implement four-level trace system`

### 验收标准

- 追踪标识在四个层级间正确传播
- 日志自动包含 context 中的追踪标识

---

## P8：场景运行器

**目标：** 串联所有核心模块实现端到端场景执行

### 任务清单

- [ ] 实现编排 DAG + 协程池 + 变量 + 生命周期 + 定时器 + 插件 + 追踪的 `Runner`
- [ ] 测试：运行简单 A→B→C 链路
- [ ] 测试：带参数关联的运行
- [ ] 测试：带条件分支的运行
- [ ] 测试：带循环的运行
- [ ] 测试：带扇出/扇入的运行
- [ ] 测试：固定时长模式运行
- [ ] 测试：固定次数模式运行
- [ ] 测试：生命周期钩子按序执行
- [ ] 测试：插件拦截请求/响应
- [ ] 测试：所有日志包含追踪标识
- [ ] 测试：context 超时取消执行
- [ ] 里程碑提交：`feat(runner): implement scene runner with full integration`

### 验收标准

- 完整 DAG 执行，所有功能协同工作
- 对模拟服务器的端到端测试成功
- 所有日志条目包含追踪标识

---

## P9：网页界面

**目标：** Vue 3 前端用于场景配置和报告查看

### 任务清单

- [ ] 初始化 Vue 3 + Vite 项目（`web/`）
- [ ] 配置 Pinia 状态管理、Vue Router、Naive UI
- [ ] 实现仪表盘页面（运行概览、实时指标）
- [ ] 实现场景编辑器页面（基于 @vue-flow/core 的 DAG 可视化编辑器）
- [ ] 实现配置页面（协程池、超时、变量、插件）
- [ ] 实现报告页面（HTML 报告、追踪链路）
- [ ] 使用 Axios 连接接口调用
- [ ] 使用 WebSocket 连接实时指标
- [ ] 编写组件测试
- [ ] 里程碑提交：`feat(web): implement Vue 3 Web UI`

### 验收标准

- 四个页面全部可用
- DAG 编辑器支持拖拽创建节点
- 实时指标通过 WebSocket 更新

---

## P10：集成与发布

**目标：** 全面集成、构建脚本、发布

### 任务清单

- [ ] 使用模拟服务器编写端到端测试套件
- [ ] 编写构建脚本（`scripts/build.sh`）
- [ ] 编写 Dockerfile
- [ ] 编写 docker-compose.yml（Salvo + MySQL + Prometheus）
- [ ] 配置文件模板（`configs/`）
- [ ] 性能基准测试
- [ ] 安全审计（代码中无密钥）
- [ ] 里程碑提交：`chore(release): bump v0.1.0`

### 验收标准

- 完整端到端测试套件通过
- Docker 构建成功
- 所有测试通过，覆盖率 ≥ 80%

---

## 进度日志

| 日期 | 阶段 | 提交 | 说明 |
|------|------|------|------|
| 2026-05-01 | — | c5b3224 | 初始化 Salvo 项目及设计规格说明书 |
| 2026-05-01 | — | 4d38e81 | 增加模拟 HTTP 服务器章节 |
