## Context

Salvo 当前架构：
- **DAG 执行器**：支持 http/delay/condition/if-else/group/timer/setup/teardown 8 种节点，sync/async 模式，条件边
- **表达式引擎**：仅 `${var}` 变量替换 + `generator.xxx` 生成器，不支持函数调用、数学运算
- **插件系统**：Plugin 接口（OnRequest/OnResponse 钩子），支持 Lua 脚本，无 .so 动态加载
- **加密模块**：`internal/plugin/crypto/` 有 AES-GCM/CBC/CTR + HMAC-SHA256，通过 Plugin 适配器集成

业务需求（来自 `docs/biz-migration/design.md` + `login.py` + `card.yaml` + `prepay.yaml`）：
- 需要 6 个系统函数：`__weightedChoice`、`__oneOf`、`__manOf`、`__Random`、`__snowflakeId`、`__so`
- 需要 4 种新节点：`while`（条件循环）、`parallel`（并行执行）、`sub_flow`（子流程）、`loop`（固定循环）
- 需要 12 个条件运算符：`equals`/`not_equals`/`greater_than`/`greater_than_or_equal`/`less_than`/`less_than_or_equal`/`not_empty`/`empty`/`size_equals`/`size_greater_than`/`size_greater_than_or_equal`/`size_less_than`
- 需要表达式支持：数学运算、嵌套函数调用、数组索引访问、字符串拼接
- 需要 SO 动态加载：Shell 登录的 AES-CBC 加解密 + BCrypt 哈希，以及未来定制化算法

现有代码基础：
- `internal/core/dag/dag.go` — DAG 数据结构 + Node 接口（ID/Execute/Timeout/LoopCount/Mode）
- `internal/runner/runner.go` — sceneNode 执行逻辑，switch case 分发节点类型
- `internal/core/variable/variable.go` — 三级作用域变量系统 + ResolveString（已支持递归嵌套）
- `internal/plugin/plugin.go` — Plugin 接口 + Registry（洋葱模型，优先级排序）
- `internal/plugin/crypto/` — AES + HMAC 加密实现 + Plugin 适配器
- `internal/generator/builtin/` — 内置生成器（email、uuid 等）
- `internal/store/model/model.go` — NodeType 常量（已定义 NodeTypeLoop 但未实现）
- `go.mod` — Go 1.26.2，模块路径 `github.com/yannick2025-tech/Salvo`

## Goals / Non-Goals

**Goals:**
- 实现表达式引擎，支持 `__` 前缀函数调用、数学运算、嵌套引用
- 实现 6 个系统函数，覆盖 design.md 中所有需求
- 实现 while/parallel/sub_flow/loop 4 种新节点
- 实现 SO 动态插件系统（Go plugin），支持版本管理、持久化、表达式集成
- 实现 SO 插件管理 GUI（管理员权限、上传/删除/废弃）
- 实现 12 个条件运算符
- 统一函数命名规范为 `__` 前缀

**Non-Goals:**
- 不实现 `__groovy` 函数（design.md 中提到但优先级低，签名机制可通过 SO 插件实现）
- 不实现 WASM 沙箱方案（未来扩展）
- 不实现 .so 热卸载（Go plugin 限制，需重启生效）
- 不实现跨平台 .so 支持（仅支持 Linux/macOS，Windows 不支持 Go plugin）
- 不实现 SO 插件的沙箱隔离（信任内部用户上传的插件）

## Decisions

### D1: 表达式引擎 — 正则替换 + 递归解析

**选择**：新建 `internal/core/expr/` 包，实现正则匹配 `${__func(args)}` + 递归解析嵌套引用。

**替代方案**：使用 ANTLR 构建完整 AST — 过度设计，当前需求是函数调用和简单数学运算，正则 + 递归足够。

**理由**：
- 与现有 `${var}` 语法兼容
- 正则匹配 `__` 前缀函数调用，剩余 `${var}` 走变量替换
- 数学运算通过 `evalGoExpr()` 实现（受限的算术表达式求值，仅支持 + - * / 和括号）
- 嵌套引用复用 `variable.ResolveString` 已有的递归解析（最多 10 层）

### D2: 函数命名规范 — 统一 `__` 前缀

**选择**：所有系统函数统一使用 `__` 前缀（JMeter 风格）。

**迁移**：现有 `generator.email` → `__email()`，`generator.uuid` → `__uuid()` 等。

**理由**：
- `__` + `()` 一眼可识别为函数调用，与 `${var}` 变量引用形成视觉区分
- JMeter/LoadRunner 用户零学习成本
- 可扩展：`__so()`、`__lua()`、`__groovy()` 等前缀统一

### D3: SO 插件接口 — 通用字符串入口 + 版本管理

**选择**：
```go
type Plugin interface {
    Name() string
    Version() string
    Init(config string) error
    Ops() []string
    Call(op string, args ...string) (string, error)
}
```

**替代方案**：强类型接口（Encrypt/Decrypt/Hash 分开）— 限制了插件只能做加密，不够通用。

**理由**：
- `Call(op, args...)` 通用入口，任何业务逻辑都能实现（加密、签名、数据转换、协议编码）
- `Version()` 支持多版本共存，`Get(name, version)` 默认取最新
- `Init(config)` 通过 JSON 字符串注入配置，灵活且与 GUI 配置面板兼容
- 日志始终打印 `name@version`，确保可追溯

### D4: SO 持久化 — 文件存储 + 启动自动加载

**选择**：
- .so 文件上传后存储到 `data/plugins/` 目录
- 插件元数据（名称、版本、路径、状态、配置）存储到 `so_plugins` 表
- Salvo 启动时查询所有 `status=enabled` 的插件，调用 `Loader.Load()` 自动加载
- 用户只需上传一次，重启后自动恢复

**替代方案**：数据库存储 .so 二进制 — SQLite 不适合存储大文件，且文件系统更适合 .so 加载。

**理由**：
- `plugin.Open()` 需要文件路径，文件存储是唯一选择
- 元数据入库便于 GUI 管理和状态追踪
- 启动自动加载对用户透明，体验好

### D5: SO 生命周期 — 不可卸载，支持废弃

**选择**：
- Go plugin 不可卸载，加载后在进程生命周期内一直有效
- 「废弃」操作：将 `so_plugins.status` 设为 `disabled`，重启后不再加载（但当前进程内仍可用）
- 「删除」操作：物理删除 .so 文件 + 数据库记录（重启后彻底消失）
- 版本更新：上传新版本 .so，旧版本仍保留在内存中直到重启

**理由**：Go `plugin` 包的设计限制，无法在运行时卸载。废弃 + 重启是最佳实践。

### D6: while 节点 — 子步骤循环 + 退出条件

**选择**：
- while 节点 config 包含 `exit_conditions`（条件列表）、`interval_seconds`（轮询间隔）、`max_iterations`（最大迭代）、`max_duration_minutes`（最大时长）、`steps`（子步骤列表）
- 执行逻辑：循环执行 steps → 检查退出条件 → 满足则退出，不满足则等待 interval_seconds 后继续
- 超过 max_iterations 或 max_duration_minutes 则强制退出并标记失败
- 支持 `fail_after_consecutive`（连续 N 次失败则终止）

**替代方案**：复用 Group + 条件边 — Group 是固定次数循环，无法实现条件退出。

**理由**：while 是业务核心需求（轮询充电状态），需要独立的条件循环语义。

### D7: parallel 节点 — goroutine 并行 + WaitGroup

**选择**：
- parallel 节点 config 包含 `steps`（子步骤列表）
- 执行逻辑：为每个子步骤启动 goroutine，WaitGroup 等待全部完成
- 任一子步骤失败不影响其他子步骤，但 parallel 节点整体返回失败

**替代方案**：复用 DAG 的 async 模式 — DAG 的 async 是节点级，无法实现一组节点的并行 + 等待。

**理由**：parallel 语义明确（首页初始化并行请求），goroutine + WaitGroup 实现简单高效。

### D8: sub_flow 节点 — 场景引用 + 异步派生

**选择**：
- sub_flow 节点 config 包含 `scene_id`（引用的场景 ID）、`async`（是否异步）
- 同步模式：等待子场景执行完成后返回
- 异步模式：启动子场景后立即返回，不阻塞主链路

**替代方案**：子 DAG 嵌套执行器 — 需要独立的子 DAG 执行器，数据模型变更大。

**理由**：引用式不需要改变 DAG 核心数据结构；异步派生适合 fire-and-forget 场景（如登录后首页）。

### D9: GUI 菜单重构

**选择**：
- 当前「系统设置」(/settings) 改名为「个人设置」
- 新增「SO 插件管理」(/plugins) 菜单项，`permission: 'plugins:read'`，仅管理员可见
- 菜单结构保持扁平（与现有风格一致）

**理由**：现有菜单是扁平结构，不引入层级菜单，保持一致性。`canAccess` 已支持 admin 角色自动放行。

### D10: 条件运算符 — 统一 evaluator

**选择**：新建 `internal/core/expr/evaluator.go`，实现 `EvaluateCondition(variable, operator, value, variables)` 函数，支持 12 个运算符。

**替代方案**：在 runner.go 中 switch case — 逻辑分散，难以测试。

**理由**：独立 evaluator 便于单元测试和复用（while 退出条件、if-else 分支、步骤执行条件都可调用）。

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        Web GUI                                    │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────────┐ │
│  │ Settings │  │ Plugins Page │  │ Scene Detail (DAG Editor)  │ │
│  │ (个人设置)│  │ (SO 插件管理) │  │ while/parallel/sub_flow   │ │
│  └──────────┘  └──────┬───────┘  └────────────────────────────┘ │
└────────────────────────┼─────────────────────────────────────────┘
                         │ REST API
┌────────────────────────┼─────────────────────────────────────────┐
│                API Layer                                         │
│  /api/v1/plugins/so/*   /api/v1/scenes/*                         │
└────────────────────────┬─────────────────────────────────────────┘
                         │
┌────────────────────────┼─────────────────────────────────────────┐
│              Core Engine                                         │
│  ┌─────────────┐  ┌────┴──────────┐  ┌────────────────────────┐ │
│  │ expr engine │  │ so plugin     │  │ runner (DAG executor)  │ │
│  │ - Resolve() │  │ - Loader      │  │ - while node           │ │
│  │ - Evaluate()│  │ - Version mgmt│  │ - parallel node        │ │
│  │ - __func()  │  │ - Persist     │  │ - sub_flow node        │ │
│  └──────┬──────┘  └───────┬───────┘  │ - loop node             │ │
│         │                 │          └────────────────────────┘ │
│  ┌──────┴──────┐  ┌───────┴───────┐                            │
│  │ builtin     │  │ so/contract   │                            │
│  │ - __Random  │  │ - Plugin iface│                            │
│  │ - __oneOf   │  │ - Factory     │                            │
│  │ - __so()    │  │ - Adapter     │                            │
│  └─────────────┘  └───────────────┘                            │
└──────────────────────────────────────────────────────────────────┘
                         │
┌────────────────────────┼─────────────────────────────────────────┐
│              Storage                                             │
│  ┌──────────────┐  ┌──────────────────┐                         │
│  │ SQLite       │  │ File System       │                         │
│  │ - so_plugins │  │ data/plugins/*.so │                         │
│  └──────────────┘  └──────────────────┘                         │
└──────────────────────────────────────────────────────────────────┘
```

## SO Plugin Interface (Final)

```go
// internal/plugin/so/contract.go
package so

// Plugin is the universal contract that .so files must implement.
// NOT crypto-specific — any custom logic can be a plugin.
type Plugin interface {
    Name() string              // unique identifier, e.g. "shell-aes"
    Version() string           // semver, e.g. "1.0.0"
    Init(config string) error  // JSON config from GUI
    Ops() []string             // supported operations, e.g. ["encrypt","decrypt"]
    Call(op string, args ...string) (string, error)  // universal entry point
}

// Factory is the exported symbol "New" that .so files must provide.
type Factory = func() interface{}
```

## Expression Syntax

```
# Variable reference (unchanged)
${token}

# System function call
${__random(60, 600)}
${__oneOf("A", "B", "C")}
${__weightedChoice("1=50,0=50")}
${__snowflakeId()}

# SO plugin call (with version)
${__so("shell-aes@1.0.0", "encrypt", "data", "key", "iv")}

# SO plugin call (latest version)
${__so("shell-aes", "encrypt", "data")}

# Math operations
${chargeTime} * ${ranking} / 100

# Nested function call
${__random(${cities.lat_min}, ${cities.lat_max})}

# Array index access
${unpaidChargeOrders[0].orderId}
```

## SO Plugin Lifecycle

```
Upload .so → Save to data/plugins/ → Insert so_plugins record (status=enabled)
    ↓
Salvo Restart → Query status=enabled → Loader.Load(path, config) → Register
    ↓
Expression Engine → ${__so("name@version", "op", "args")} → Loader.Get(name, version) → Plugin.Call(op, args)
    ↓
Log: [so] call success: plugin=name version=x.x.x op=xxx

Disable (废弃) → Update status=disabled → Restart → Not loaded
Delete (删除) → Delete .so file + DB record → Restart → Gone
```
