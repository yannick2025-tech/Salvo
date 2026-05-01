# salvo 设计规格说明书

> 基于 DAG 工作流引擎的 HTTP 性能测试工具

**许可证：** GNU AGPL v3  
**日期：** 2026-04-30  
**状态：** 已确认

---

## 1. 概述

salvo 是一个基于 Go 构建的 HTTP 性能测试工具，具有 DAG 工作流引擎、插件系统和 Vue 3 网页界面。支持链路式接口测试、参数关联、生命周期管理，以及可扩展协议设计（未来支持数据库/FTP）。

### 1.1 核心需求

- DAG 工作流：同步/异步调用、条件分支、循环
- 参数关联（B 的响应用于 C 的请求参数）
- 三级变量作用域：全局 → 场景 → 接口
- 生命周期管理：全局初始化/清理 + 场景初始化/清理
- 固定协程池（可配置，如 20 协程跑 1000 万请求）
- 两种运行模式：固定时长（36小时）或固定次数（1000 万次）
- K8S 风格插件系统（Go 内置 + Lua 自定义）
- JSON Schema Draft 7 参数生成器
- 定时器类型：周期定时器、思考时间、随机延迟
- 四级追踪：场景 → 链路 → 接口 → 函数
- 网页界面用于场景配置和报告查看
- 所有接口端点使用 POST 方法
- TDD 驱动开发

### 1.2 非目标

- 分布式执行（未来考虑）
- 场景编辑的实时协作
- 移动端界面

---

## 2. 架构

### 2.1 架构风格：单体分层

单一 Go 二进制，内部分层。模块通过 Go 接口通信。由于边界是接口驱动的，未来可拆分为微服务。

### 2.2 项目结构

```
salvo/
├── cmd/
│   └── salvo/              # 入口 main.go
├── internal/
│   ├── core/                # 核心引擎
│   │   ├── dag/             # DAG 定义、解析、执行
│   │   ├── pool/            # 协程池
│   │   ├── contextx/        # Context 管理（超时级联）
│   │   ├── variable/        # 变量系统（全局/场景/接口）
│   │   ├── lifecycle/       # 生命周期（初始化/清理）
│   │   ├── timer/           # 定时器（周期/思考时间/随机）
│   │   └── runner/          # 场景运行器（串联以上模块）
│   ├── protocol/            # 协议抽象层
│   │   ├── protocol.go      # Protocol 接口定义
│   │   ├── http/            # HTTP 协议实现
│   │   ├── db/              # 数据库协议实现（未来）
│   │   └── ftp/             # FTP 协议实现（未来）
│   ├── plugin/              # 插件系统
│   │   ├── registry.go      # 插件注册中心
│   │   ├── ratelimit/       # 限速插件（内置）
│   │   ├── crypto/          # 加解密插件（内置）
│   │   ├── report/          # 报告插件（内置）
│   │   │   ├── html/        # HTML 报告
│   │   │   └── prometheus/  # Prometheus 指标
│   │   └── lua/             # Lua 脚本引擎
│   ├── generator/           # 参数生成器
│   │   ├── schema/          # JSON Schema 解析与生成
│   │   └── builtin/         # 内置生成器
│   ├── api/                 # REST 接口层
│   │   ├── handler/         # HTTP 处理器
│   │   ├── middleware/      # 中间件
│   │   └── dto/             # 请求/响应数据传输对象
│   ├── store/               # 数据存储层
│   │   ├── model/           # 数据模型
│   │   ├── repo/            # Repository 接口
│   │   └── migration/       # 数据库迁移
│   └── logger/              # 日志系统
│       ├── logger.go        # 日志接口与工厂
│       ├── zap.go           # Zap 实现
│       └── middleware.go    # HTTP 日志中间件
├── web/                     # Vue 3 前端
│   ├── src/
│   │   ├── views/           # 页面
│   │   ├── components/      # 组件
│   │   ├── composables/     # 组合式函数
│   │   └── stores/          # Pinia 状态管理
│   └── ...
├── configs/                 # 配置文件模板
├── scripts/                 # 构建/部署脚本
├── docs/                    # 文档
└── test/                    # 集成测试/端到端测试
```

### 2.3 分层依赖规则

```
Vue 3 网页界面（展示层）
    ↓ 只调接口层
REST 接口层
    ↓ 调核心层 + 插件层
核心引擎（核心层）
    ↓ 调插件层 + 存储层
插件 + 协议 + 生成器（扩展层）
    ↓ 无上层依赖
日志 + 存储（基础设施层）
    ↓ 无上层依赖
```

上层依赖下层，下层不知道上层存在。模块通过 Go 接口通信。

### 2.4 设计原则

1. **接口驱动**：核心层依赖 Protocol 接口，不依赖 HTTP 包
2. **依赖注入**：构造函数注入，不用全局变量，便于模拟
3. **Context 传播**：所有异步操作通过 context.Context 控制
4. **TDD 优先**：先写测试再写实现

---

## 3. 核心引擎

### 3.1 DAG 执行器

```go
type ExecMode int
const (
    ExecSync  ExecMode = iota  // wait for response
    ExecAsync                   // fire and forget
)

type Node interface {
    ID() string
    Execute(ctx context.Context, input *Input) (*Output, error)
    Timeout() time.Duration
    LoopCount() int
    Mode() ExecMode
}

type Edge struct {
    From      string
    To        string
    Condition string  // expression, empty = unconditional
}

type DAG interface {
    AddNode(node Node)
    AddEdge(edge Edge)
    Validate() error              // check acyclic
    TopologicalSort() ([]Node, error)
    RootNodes() []Node
}
```

**DAG 执行语义：**
- 同步节点：等待响应后再继续
- 异步节点：触发后继续
- 条件边：对变量存储求值表达式
- 循环：节点级配置，不是图的环
- 扇出：并行执行多个子节点
- 扇入：等待所有父节点完成

### 3.2 协程池

```go
type Pool interface {
    Submit(ctx context.Context, task Task) error
    SubmitAndWait(ctx context.Context, task Task) (*Result, error)
    Shutdown(ctx context.Context) error
    Running() int
    Waiting() int
}

type Task func(ctx context.Context) (*Result, error)

type RunMode int
const (
    RunModeDuration RunMode = iota  // run for specified duration
    RunModeCount                     // run specified number of iterations
)
```

- 固定池大小（可配置，默认 20）
- 持续从队列消费任务
- 两种运行模式：固定时长或固定次数
- 通过 context 优雅关闭

### 3.3 Context 超时级联

```
全局 Context (WithCancel) → 取消所有场景
  └── 场景 Context (WithTimeout) → 场景级超时
        └── 节点 Context (WithTimeout) → 节点级超时
```

- 子超时不影响父
- 父取消则所有子取消
- 所有超时可通过 YAML 和网页界面配置

### 3.4 变量系统

```go
type Scope int
const (
    ScopeGlobal  Scope = iota  // cross-scene, lifetime = app
    ScopeScene                  // within a scene, lifetime = scene run
    ScopeAPI                    // within a node, lifetime = node execution
)

type Store interface {
    Set(scope Scope, key string, value any)
    Get(scope Scope, key string) (any, bool)
    Resolve(expr string) (any, error)          // "${A.token}" → actual value
    ResolveAll(params map[string]any) (map[string]any, error)
}

// Lookup order: API → Scene → Global (inner scope overrides outer)
```

参数关联：`${A.token}` 解析为节点 A 响应中的 `token` 字段。解析在请求构建时发生。

### 3.5 生命周期管理

```
全局初始化（执行一次）
  → 场景初始化（每个场景执行一次）
    → DAG 执行（每次迭代执行）
  → 场景清理（每个场景执行一次）
全局清理（执行一次）
```

每个生命周期钩子可以执行任意协议调用并设置变量。

### 3.6 定时器

```go
type TimerType int
const (
    TimerTicker    TimerType = iota  // every X seconds
    TimerThinkTime                    // wait X seconds then execute
    TimerRandom                       // random delay in [min, max]
)

type Timer interface {
    Wait(ctx context.Context) error  // blocks until next tick, respects ctx cancel
    Reset()
}
```

- **周期定时器**：每 X 秒周期性执行（如心跳）
- **思考时间**：等待 X 秒后执行（如模拟用户思考时间）
- **随机延迟**：[最小值, 最大值] 区间随机等待（如模拟真实用户差异）

---

## 4. 协议层

### 4.1 协议接口

```go
type Protocol interface {
    Name() string
    Execute(ctx context.Context, req *Request) (*Response, error)
    Validate(req *Request) error
}

type Request struct {
    Method  string
    URL     string
    Headers map[string]string
    Body    any
    Params  map[string]string
}

type Response struct {
    StatusCode int
    Headers    map[string]string
    Body       any
    Latency    time.Duration
    Error      error
}
```

### 4.2 新增协议指南

新增协议（如数据库、FTP、gRPC）需实现以下内容：

1. 在 `internal/protocol/<名称>/` 中实现 `Protocol` 接口
2. 在 `internal/protocol/registry.go` 中注册协议
3. 为新协议的连接参数添加 YAML 配置模式
4. 在 DAG 编辑器中添加新协议的网页界面节点类型
5. 如果协议有特定参数类型，添加生成器支持
6. 添加测试：协议实现的单元测试 + 与 DAG 运行器的集成测试

---

## 5. 插件系统

### 5.1 插件接口

```go
type Plugin interface {
    Name() string
    Init(ctx context.Context, config map[string]any) error
    OnRequest(ctx context.Context, req *Request) (*Request, error)
    OnResponse(ctx context.Context, resp *Response) (*Response, error)
    Shutdown(ctx context.Context) error
}

type HookPoint int
const (
    HookBeforeRequest  HookPoint = iota
    HookAfterResponse
    HookOnSetup
    HookOnTeardown
    HookOnError
)

type Registry interface {
    Register(plugin Plugin, hooks ...HookPoint)
    GetPlugins(hook HookPoint) []Plugin
    LoadLuaPlugin(scriptPath string, config map[string]any) error
}
```

### 5.2 内置插件

#### 5.2.1 限速插件

- 全局每秒请求数限制（令牌桶算法）
- 单 URL 每秒请求数限制
- 配置：`global_qps`、`url_limits`
- 钩子：`HookBeforeRequest`

#### 5.2.2 加解密插件

内置算法：
- 哈希：MD5、SHA256
- 对称加密：AES、DES
- 非对称加密：RSA、DSA
- 密码哈希：BCRYPT

钩子：`HookBeforeRequest`（加密）、`HookAfterResponse`（解密）

自定义算法：Lua 脚本可覆盖内置实现。

#### 5.2.3 报告插件

- HTML 报告生成
- Prometheus 指标端点（可配置 IP 和端口）
- 测试执行期间实时推送指标

### 5.3 Lua 自定义插件

```lua
-- plugins/custom_encrypt.lua
local plugin = {
    name = "custom_encrypt",
}

function plugin.on_request(req)
    local key = salvo.get_var("api", "encrypt_key")
    req.body = my_encrypt(req.body, key)
    return req
end

function plugin.on_response(resp)
    local key = salvo.get_var("api", "decrypt_key")
    resp.body = my_decrypt(resp.body, key)
    return resp
end

return plugin
```

Lua 插件可访问：
- `salvo.get_var(scope, key)` — 读取变量
- `salvo.set_var(scope, key, value)` — 设置变量
- `salvo.log(level, message)` — 结构化日志
- `salvo.http_request(method, url, body)` — 发起 HTTP 请求

---

## 6. JSON Schema 参数生成器

### 6.1 架构

三种模式来源：
1. Swagger/OpenAPI 规范
2. JSON Schema Draft 7
3. 手动配置

所有来源归一化为统一的内部模式表示，然后输入生成器。

### 6.2 生成器接口

```go
type Generator interface {
    Generate(schema *Schema) (any, error)
    CanHandle(schema *Schema) bool
}
```

### 6.3 内置生成器

| JSON Schema 类型 | 生成器 | 说明 |
|-----------------|--------|------|
| string | RandomString | 随机字母数字 |
| string + pattern | RegexString | 基于正则生成 |
| string + enum | EnumString | 从枚举值中选取 |
| string + format=uuid | UUIDGenerator | UUID v4 |
| string + format=email | EmailGenerator | 随机邮箱 |
| string + format=date | DateGenerator | 日期字符串 |
| number | RandomFloat | [最小值, 最大值] 区间随机浮点数 |
| integer | RandomInt | [最小值, 最大值] 区间随机整数 |
| integer | IncrementInt | 顺序递增 |
| boolean | RandomBool | 50/50 随机 |
| boolean | WeightedBool | 可配置真值比例 |
| array | ArrayGenerator | 嵌套元素，最小/最大/唯一 |
| object | ObjectGenerator | 属性，必填字段 |
| null | NullGenerator | 空值 |

### 6.4 支持的 JSON Schema Draft 7 关键字

enum, minLength, maxLength, pattern, format, minimum, maximum, exclusiveMinimum, exclusiveMaximum, multipleOf, minItems, maxItems, uniqueItems, properties, required, additionalProperties, allOf, anyOf, oneOf, const, default

### 6.5 自定义生成器

自定义生成器通过 Lua 脚本实现 `Generator` 接口。注册方式与自定义加密函数相同。

---

## 7. 数据存储

### 7.1 Repository 模式

```go
type SceneRepo interface {
    Create(ctx context.Context, scene *model.Scene) error
    GetByID(ctx context.Context, id string) (*model.Scene, error)
    List(ctx context.Context, filter Filter) ([]*model.Scene, error)
    Update(ctx context.Context, scene *model.Scene) error
    Delete(ctx context.Context, id string) error  // soft delete
}
```

### 7.2 数据库支持

- **MySQL** — 生产首选
- **PostgreSQL** — 复杂查询场景
- **SQLite** — 本地开发/测试

对象关系映射：`ent`（代码生成式，类型安全，多方言支持）  
迁移：`golang-migrate`

### 7.3 数据模型规范

```go
type Model struct {
    ID        SnowflakeID  `json:"id,string"`           // snowflake ID, string to avoid JS precision loss
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
    DeletedAt *time.Time   `json:"deleted_at,omitempty"` // soft delete
}

type SnowflakeID int64

func (id SnowflakeID) MarshalJSON() ([]byte, error) {
    return []byte(`"` + strconv.FormatInt(int64(id), 10) + `"`), nil
}

func (id *SnowflakeID) UnmarshalJSON(data []byte) error {
    str := strings.Trim(string(data), `"`)
    val, err := strconv.ParseInt(str, 10, 64)
    if err != nil { return err }
    *id = SnowflakeID(val)
    return nil
}
```

**关键规则：**
- 所有主键使用雪花算法
- JSON 序列化始终使用字符串防止 JavaScript float64 精度丢失
- 所有表使用软删除（`deleted_at` 字段）
- Repository 层统一过滤软删除记录

---

## 8. 日志系统

### 8.1 架构

基于 Zap 结构化日志，自动注入追踪标识。

### 8.2 输出格式

**文本格式**（控制台，人类可读）：
```
2026-04-30T10:15:30.123Z  INFO  runner.scene  trace_id=abc123 scene_id=s1  node=A status=ok latency=45ms
```

**JSON 格式**（结构化，接入 ELK/Loki）：
```json
{
  "ts": "2026-04-30T10:15:30.123Z",
  "level": "info",
  "logger": "runner.scene",
  "trace_id": "abc123",
  "scene_id": "s1",
  "node": "A",
  "status": "ok",
  "latency_ms": 45
}
```

### 8.3 配置

- `format`：text | json
- `level`：debug | info | warn | error
- `output`：stdout | file | both
- 可通过 YAML 和网页界面配置

---

## 9. 追踪系统

### 9.1 四级追踪

```
场景追踪 (trace_id)
  └── 链路追踪 (span_id, parent_id = 场景 trace_id)
        └── 接口追踪 (span_id, parent_id = 链路 span_id)
              └── 函数追踪 (span_id, parent_id = 接口 span_id)
```

### 9.2 追踪接口

```go
type Span struct {
    TraceID   string
    SpanID    string
    ParentID  string
    Name      string
    StartTime time.Time
    Duration  time.Duration
    Tags      map[string]string
    Status    SpanStatus  // OK | Error | Timeout
}

type Tracer interface {
    StartSpan(ctx context.Context, name string) (context.Context, *Span)
    FinishSpan(span *Span)
    SpanFromContext(ctx context.Context) *Span
    InjectTraceID(logger *zap.Logger, ctx context.Context) *zap.Logger
}
```

### 9.3 追踪标识传播

- 场景运行 → 追踪标识注入 context
- 链路迭代 → 新 span，父级 = 场景追踪
- 接口调用 → 新 span，父级 = 链路追踪
- 插件/生成器调用 → 新 span，父级 = 接口追踪
- 日志自动包含 context 中的追踪标识

---

## 10. REST 接口

所有接口端点使用 POST 方法。查询参数通过请求体传递。

### 10.1 场景管理

```
POST /api/v1/scenes/list          # 列出场景（过滤条件在请求体中）
POST /api/v1/scenes/create        # 创建场景
POST /api/v1/scenes/get           # 获取场景详情 {id: "..."}
POST /api/v1/scenes/update        # 更新场景
POST /api/v1/scenes/delete        # 软删除场景 {id: "..."}
```

### 10.2 场景执行

```
POST /api/v1/scenes/run           # 开始测试运行 {id: "...", config: {...}}
POST /api/v1/scenes/stop          # 停止测试运行 {id: "..."}
POST /api/v1/scenes/status        # 实时状态 {id: "..."}
```

### 10.3 DAG 节点

```
POST /api/v1/scenes/nodes/list    # 列出节点 {scene_id: "..."}
POST /api/v1/scenes/nodes/add     # 添加节点
POST /api/v1/scenes/nodes/update  # 更新节点
POST /api/v1/scenes/nodes/delete  # 删除节点
```

### 10.4 DAG 边

```
POST /api/v1/scenes/edges/add     # 添加边
POST /api/v1/scenes/edges/delete  # 删除边
```

### 10.5 报告

```
POST /api/v1/reports/list         # 列出报告
POST /api/v1/reports/get          # 获取报告详情
POST /api/v1/reports/html         # 下载 HTML 报告
```

### 10.6 插件

```
POST /api/v1/plugins/list         # 列出已注册插件
POST /api/v1/plugins/config       # 更新插件配置
```

### 10.7 变量

```
POST /api/v1/scenes/variables/list   # 列出场景变量
POST /api/v1/scenes/variables/set    # 设置场景变量
```

### 10.8 WebSocket

```
WS /api/v1/ws/run/:id             # 实时指标流
```

---

## 11. 网页界面

### 11.1 技术栈

- Vue 3 + Composition API
- Pinia（状态管理）
- Vue Router
- Vite（构建工具）
- Naive UI（组件库）
- @vue-flow/core（DAG 可视化编辑器）
- ECharts（实时图表）
- Axios + WebSocket（接口 + 流式推送）

### 11.2 页面

| 页面 | 说明 |
|------|------|
| 仪表盘 | 运行概览，实时指标 |
| 场景编辑器 | DAG 可视化编排，节点/边配置 |
| 配置页 | 协程池/超时/变量，插件开关/配置 |
| 报告页 | HTML 报告查看，追踪链路检查 |

### 11.3 设计风格

现代互联网风格界面，类似 Grafana/Kibana 的美学。

---

## 12. 配置

所有可配置项同时支持 YAML 文件和网页界面配置。

### 12.1 配置项

| 分类 | 配置项 |
|------|--------|
| 协程池 | pool_size, run_mode, duration, count |
| 超时 | global_timeout, scene_timeout, node_timeout（每个节点） |
| 定时器 | type（周期/思考时间/随机）, interval, min, max |
| 变量 | global_vars, scene_vars, api_vars |
| 插件 | enabled, 各插件配置 |
| 限速 | global_qps, url_limits |
| 加解密 | algorithm, key, mode |
| 报告 | format, prometheus_ip, prometheus_port |
| 日志 | format（text/json）, level, output |
| 数据库 | dialect, dsn, max_connections |

### 12.2 YAML 示例

```yaml
engine:
  pool_size: 20
  run_mode: count        # duration | count
  duration: 36h
  count: 10000000

timeout:
  global: 0              # 0 = no global timeout
  scene: 60s
  nodes:
    A: 5s
    B: 3s
    C: 10s

timer:
  type: thinktime        # ticker | thinktime | random
  interval: 5s
  min: 1s
  max: 10s

plugins:
  ratelimit:
    enabled: true
    global_qps: 1000
    url_limits:
      "/api/login": 100
  crypto:
    enabled: true
    algorithm: aes
    key: "${global.aes_key}"
  report:
    html:
      enabled: true
    prometheus:
      enabled: true
      ip: "0.0.0.0"
      port: 9090

logger:
  format: json           # text | json
  level: info
  output: both           # stdout | file | both

database:
  dialect: mysql
  dsn: "user:pass@tcp(localhost:3306)/salvo"
  max_connections: 20
```

---

## 13. TDD 策略

### 13.1 测试金字塔

- **70% 单元测试** — 接口模拟实现，表驱动测试
- **20% 集成测试** — 模块间交互（DAG+协程池+变量）
- **10% 端到端测试** — 完整场景 HTTP 测试

### 13.2 测试工具链

| 工具 | 用途 |
|------|------|
| testing | 标准库，表驱动测试 |
| testify/assert | 流式断言 |
| testify/mock | 接口模拟 |
| testcontainers | MySQL/PG 集成测试 |
| httptest | HTTP 处理器测试 |
| go test -race | 竞态检测 |

### 13.3 覆盖率目标

≥ 80% 行覆盖率

### 13.4 TDD 工作流

1. 红灯 — 先写失败测试
2. 绿灯 — 最小实现使测试通过
3. 重构 — 重构，测试保持绿灯
4. 覆盖率 — 验证覆盖率达标

---

## 14. Git 规范

### 14.1 分支策略

| 分支 | 用途 |
|------|------|
| `main` | 生产分支，只合并发布 |
| `develop` | 开发主线，每日集成 |
| `feature/*` | 功能分支，完成后合并回 develop |
| `hotfix/*` | 紧急修复，从 main 分出 |

### 14.2 提交信息格式

```
<类型>(<范围>): <主题>

[可选正文]

[可选页脚: BREAKING CHANGE | Closes #xxx]
```

### 14.3 类型

| 类型 | 说明 |
|------|------|
| feat | 新功能 |
| fix | 缺陷修复 |
| refactor | 代码重构（无功能/修复） |
| test | 添加/更新测试 |
| docs | 文档 |
| perf | 性能优化 |
| chore | 构建/配置/工具 |
| style | 格式化，无逻辑变更 |

### 14.4 范围

dag, pool, variable, lifecycle, timer, plugin, generator, protocol, api, store, logger, trace, web, config, crypto, report

### 14.5 里程碑提交策略

每个关键模块必须通过所有测试后才能提交：

1. **接口定义** — `type(范围): 定义 xxx 接口` → 测试：模拟实现 + 接口合规
2. **核心实现** — `feat(范围): 实现 xxx` → 测试：表驱动 + 边界用例
3. **集成** — `feat(范围): 集成 xxx 与 yyy` → 测试：集成测试
4. **发布** — `chore(release): 升级 v0.x.0` → 所有测试通过 + 覆盖率达标

---

## 15. 模拟 HTTP 服务器

内置模拟 HTTP 服务器，用于本地端到端测试，位于 `test/mockserver/`。

### 15.1 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/login | 返回令牌，模拟认证 |
| POST | /api/users | 创建用户，返回用户 JSON |
| GET | /api/users/:id | 按 ID 获取用户 |
| GET | /api/users | 分页列出用户 |
| PUT | /api/users/:id | 更新用户 |
| DELETE | /api/users/:id | 删除用户 |
| POST | /api/orders | 创建订单 |
| GET | /api/orders | 列出订单 |
| POST | /api/upload | 上传文件，返回文件信息 |
| GET | /api/delay/:ms | 延迟响应（可配置延迟时间） |
| GET | /api/status/:code | 返回指定 HTTP 状态码 |
| POST | /api/echo | 回显请求体 |
| GET | /api/headers | 回显请求头 |
| POST | /api/encrypt | 接收加密请求体，返回加密响应 |
| GET | /api/chunked | 分块传输编码响应 |
| GET | /api/redirect/:count | 链式重定向 |
| POST | /api/error | 随机服务端错误（500/502/503） |

### 15.2 功能特性

- 可配置响应延迟（模拟慢速接口）
- 可配置错误率（模拟不稳定服务）
- 请求日志（检查 Salvo 发出的请求内容）
- 启用 CORS（方便网页界面测试）
- 通过 `go test` 或独立二进制启动

### 15.3 使用方式

```go
func TestE2ELoginFlow(t *testing.T) {
    srv := mockserver.New(t, mockserver.Config{
        Port:     18080,
        Latency:  50 * time.Millisecond,
        ErrorRate: 0.0,
    })
    defer srv.Close()
    
    // 测试目标: srv.URL() + "/api/login"
}
```

---

## 16. 代码规范

- 所有代码注释使用英文
- 文档同时提供中文和英文（独立文件）
- 许可证：GNU AGPL v3
- 关键功能先设计后开发
- 扩展指南需文档化（如"如何添加新协议"）

---

## 17. 使用的技能

| 技能 | 用途 |
|------|------|
| golang-pro | Go 开发规范，并发模式 |
| sql-pro | 数据库模式设计，查询优化 |
| vue-expert | Vue 3 + Composition API 开发 |
| vue-expert-js | Vue 3 JavaScript 组合式函数 |
| api-designer | REST 接口设计与规范 |
| frontend-design | 网页界面设计与实现 |
