# salvo Design Specification

> HTTP Performance Testing Tool with DAG Workflow Engine

**License:** GNU AGPL v3  
**Date:** 2026-04-30  
**Status:** Approved

---

## 1. Overview

salvo is an HTTP performance testing tool built in Go, featuring a DAG-based workflow engine, plugin system, and Vue 3 Web UI. It supports chain-style API testing with parameter correlation, lifecycle management, and extensible protocol design for future DB/FTP support.

### 1.1 Key Requirements

- DAG workflow with sync/async calls, conditional branching, loops
- Parameter correlation (response of B feeds into request of C)
- Three-level variable scope: Global → Scene → API
- Lifecycle management: Global Setup/Teardown + Scene Setup/Teardown
- Fixed goroutine pool (configurable, e.g., 20 goroutines for 10M requests)
- Two run modes: duration-based (36h) or count-based (10M iterations)
- K8S-style plugin system (Go built-in + Lua custom)
- JSON Schema Draft 7 parameter generator
- Timer types: Ticker, ThinkTime, Random
- Four-level trace: Scene → Chain → API → Function
- Web UI for scenario configuration and report viewing
- All API endpoints use POST method
- TDD-driven development

### 1.2 Non-Goals

- Distributed execution (future consideration)
- Real-time collaboration on scenario editing
- Mobile UI

---

## 2. Architecture

### 2.1 Architecture Style: Monolithic Layered

Single Go binary with internal layering. Modules communicate through Go interfaces. Future decomposition into microservices is possible since boundaries are interface-driven.

### 2.2 Project Structure

```
salvo/
├── cmd/
│   └── salvo/              # Entry point main.go
├── internal/
│   ├── core/                # Core engine
│   │   ├── dag/             # DAG definition, parsing, execution
│   │   ├── pool/            # Goroutine pool
│   │   ├── contextx/        # Context management (timeout cascade)
│   │   ├── variable/        # Variable system (Global/Scene/API)
│   │   ├── lifecycle/       # Lifecycle (Setup/Teardown)
│   │   ├── timer/           # Timers (Ticker/ThinkTime/Random)
│   │   └── runner/          # Scene runner (orchestrates above modules)
│   ├── protocol/            # Protocol abstraction layer
│   │   ├── protocol.go      # Protocol interface definition
│   │   ├── http/            # HTTP protocol implementation
│   │   ├── db/              # DB protocol implementation (future)
│   │   └── ftp/             # FTP protocol implementation (future)
│   ├── plugin/              # Plugin system
│   │   ├── registry.go      # Plugin registry
│   │   ├── ratelimit/       # Rate limiter plugin (built-in)
│   │   ├── crypto/          # Crypto plugin (built-in)
│   │   ├── report/          # Report plugin (built-in)
│   │   │   ├── html/        # HTML report
│   │   │   └── prometheus/  # Prometheus metrics
│   │   └── lua/             # Lua script engine
│   ├── generator/           # Parameter generator
│   │   ├── schema/          # JSON Schema parsing & generation
│   │   └── builtin/         # Built-in generators
│   ├── api/                 # REST API layer
│   │   ├── handler/         # HTTP handlers
│   │   ├── middleware/      # Middleware
│   │   └── dto/             # Request/Response DTOs
│   ├── store/               # Data storage layer
│   │   ├── model/           # Data models
│   │   ├── repo/            # Repository interfaces
│   │   └── migration/       # Database migrations
│   └── logger/              # Logging system
│       ├── logger.go        # Logger interface & factory
│       ├── zap.go           # Zap implementation
│       └── middleware.go    # HTTP logging middleware
├── web/                     # Vue 3 frontend
│   ├── src/
│   │   ├── views/           # Pages
│   │   ├── components/      # Components
│   │   ├── composables/     # Composables
│   │   └── stores/          # Pinia state management
│   └── ...
├── configs/                 # Config file templates
├── scripts/                 # Build/deploy scripts
├── docs/                    # Documentation
└── test/                    # Integration/E2E tests
```

### 2.3 Layer Dependency Rules

```
Vue 3 Web UI (Presentation Layer)
    ↓ only calls API
REST API Layer (Interface Layer)
    ↓ calls Core + Plugin
Core Engine (Core Layer)
    ↓ calls Plugin + Store
Plugin + Protocol + Generator (Extension Layer)
    ↓ no upper dependency
Logger + Store (Infrastructure Layer)
    ↓ no upper dependency
```

Upper layers depend on lower layers. Lower layers do not know about upper layers. Modules communicate through Go interfaces.

### 2.4 Design Principles

1. **Interface-Driven**: Core depends on Protocol interface, not HTTP package
2. **Dependency Injection**: Constructor injection, no global variables, easy to mock
3. **Context Propagation**: All async operations controlled via context.Context
4. **TDD First**: Write tests before implementation

---

## 3. Core Engine

### 3.1 DAG Executor

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

**DAG execution semantics:**
- Sync nodes: wait for response before proceeding to next
- Async nodes: fire and continue
- Conditional edges: evaluate expression against variable store
- Loop: node-level configuration, not graph cycle
- Fan-out: parallel execution of multiple children
- Fan-in: wait for all parents to complete

### 3.2 Goroutine Pool

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

- Fixed pool size (configurable, default 20)
- Continuously consumes tasks from queue
- Two run modes: duration-based or count-based
- Graceful shutdown via context

### 3.3 Context Timeout Cascade

```
Global Context (WithCancel) → cancel all scenes
  └── Scene Context (WithTimeout) → scene-level timeout
        └── Node Context (WithTimeout) → node-level timeout
```

- Child timeout does not affect parent
- Parent cancel cancels all children
- All timeouts configurable via YAML and Web UI

### 3.4 Variable System

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

Parameter correlation: `${A.token}` resolves to the `token` field from node A's response. Resolution happens at request construction time.

### 3.5 Lifecycle Management

```
Global Setup (once)
  → Scene Setup (per scene)
    → DAG Execution (per iteration)
  → Scene Teardown (per scene)
Global Teardown (once)
```

Each lifecycle hook can execute arbitrary protocol calls and set variables.

### 3.6 Timer

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

- **Ticker**: periodic execution every X seconds (e.g., heartbeat)
- **ThinkTime**: wait X seconds before execution (e.g., simulate user think time)
- **Random**: random delay in [min, max] range (e.g., simulate real user variance)

---

## 4. Protocol Layer

### 4.1 Protocol Interface

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

### 4.2 Adding a New Protocol

To add a new protocol (e.g., DB, FTP, gRPC), implement the following:

1. **Implement `Protocol` interface** in `internal/protocol/<name>/`
2. **Register protocol** in `internal/protocol/registry.go`
3. **Add YAML config schema** for the new protocol's connection parameters
4. **Add Web UI node type** in the DAG editor for the new protocol
5. **Add generator support** if the protocol has specific parameter types
6. **Add tests**: unit tests for protocol implementation + integration tests with DAG runner

---

## 5. Plugin System

### 5.1 Plugin Interface

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

### 5.2 Built-in Plugins

#### 5.2.1 RateLimiter Plugin

- Global QPS limit (token bucket algorithm)
- Per-URL QPS limit
- Config: `global_qps`, `url_limits` map
- Hook: `HookBeforeRequest`

#### 5.2.2 Crypto Plugin

Built-in algorithms:
- Hash: MD5, SHA256
- Symmetric: AES, DES
- Asymmetric: RSA, DSA
- Password: BCRYPT

Hook: `HookBeforeRequest` (encrypt), `HookAfterResponse` (decrypt)

Custom algorithms: Lua script can override built-in implementations.

#### 5.2.3 Report Plugin

- HTML report generation
- Prometheus metrics endpoint (configurable IP and port)
- Real-time metrics push during test execution

### 5.3 Lua Custom Plugins

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

Lua plugins have access to:
- `salvo.get_var(scope, key)` — read variables
- `salvo.set_var(scope, key, value)` — set variables
- `salvo.log(level, message)` — structured logging
- `salvo.http_request(method, url, body)` — make HTTP requests

---

## 6. JSON Schema Parameter Generator

### 6.1 Architecture

Three schema sources:
1. Swagger/OpenAPI spec
2. JSON Schema Draft 7
3. Manual configuration

All sources are normalized into a unified internal schema representation, then fed to generators.

### 6.2 Generator Interface

```go
type Generator interface {
    Generate(schema *Schema) (any, error)
    CanHandle(schema *Schema) bool
}
```

### 6.3 Built-in Generators

| JSON Schema Type | Generator | Description |
|-----------------|-----------|-------------|
| string | RandomString | random alphanumeric |
| string + pattern | RegexString | regex-based generation |
| string + enum | EnumString | pick from enum values |
| string + format=uuid | UUIDGenerator | UUID v4 |
| string + format=email | EmailGenerator | random email |
| string + format=date | DateGenerator | date string |
| number | RandomFloat | random float in [min, max] |
| integer | RandomInt | random int in [min, max] |
| integer | IncrementInt | sequential increment |
| boolean | RandomBool | 50/50 random |
| boolean | WeightedBool | configurable true ratio |
| array | ArrayGenerator | nested items, min/max/unique |
| object | ObjectGenerator | properties, required fields |
| null | NullGenerator | null value |

### 6.4 Supported JSON Schema Draft 7 Keywords

enum, minLength, maxLength, pattern, format, minimum, maximum, exclusiveMinimum, exclusiveMaximum, multipleOf, minItems, maxItems, uniqueItems, properties, required, additionalProperties, allOf, anyOf, oneOf, const, default

### 6.5 Custom Generators

Custom generators are implemented as Lua scripts that implement the `Generator` interface. They are registered the same way as custom crypto functions.

---

## 7. Data Storage

### 7.1 Repository Pattern

```go
type SceneRepo interface {
    Create(ctx context.Context, scene *model.Scene) error
    GetByID(ctx context.Context, id string) (*model.Scene, error)
    List(ctx context.Context, filter Filter) ([]*model.Scene, error)
    Update(ctx context.Context, scene *model.Scene) error
    Delete(ctx context.Context, id string) error  // soft delete
}
```

### 7.2 Database Support

- **MySQL** — production default
- **PostgreSQL** — for complex query scenarios
- **SQLite** — local dev/testing

ORM: `ent` (code-gen, type-safe, multi-dialect)  
Migration: `golang-migrate`

### 7.3 Data Model Standards

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

**Key rules:**
- All primary keys use Snowflake algorithm
- JSON serialization always uses string to prevent JavaScript float64 precision loss
- All tables use soft delete (`deleted_at` field)
- Repository layer uniformly filters soft-deleted records

---

## 8. Logging System

### 8.1 Architecture

Based on Zap structured logger with automatic trace_id injection.

### 8.2 Output Formats

**TEXT format** (console, human-readable):
```
2026-04-30T10:15:30.123Z  INFO  runner.scene  trace_id=abc123 scene_id=s1  node=A status=ok latency=45ms
```

**JSON format** (structured, ELK/Loki ready):
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

### 8.3 Configuration

- `format`: text | json
- `level`: debug | info | warn | error
- `output`: stdout | file | both
- Configurable via YAML and Web UI

---

## 9. Trace System

### 9.1 Four-Level Trace

```
Scene Trace (trace_id)
  └── Chain Trace (span_id, parent_id = scene trace_id)
        └── API Trace (span_id, parent_id = chain span_id)
              └── Function Trace (span_id, parent_id = api span_id)
```

### 9.2 Trace Interface

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

### 9.3 TraceID Propagation

- Scene run → trace_id injected into context
- Chain iteration → new span with parent = scene trace
- API call → new span with parent = chain span
- Plugin/Generator call → new span with parent = API span
- Logger automatically includes trace_id from context

---

## 10. REST API

All endpoints use POST method. Query parameters are passed via request body.

### 10.1 Scene Management

```
POST /api/v1/scenes/list          # list scenes (filter in body)
POST /api/v1/scenes/create        # create scene
POST /api/v1/scenes/get           # get scene detail {id: "..."}
POST /api/v1/scenes/update        # update scene
POST /api/v1/scenes/delete        # soft delete scene {id: "..."}
```

### 10.2 Scene Execution

```
POST /api/v1/scenes/run           # start test run {id: "...", config: {...}}
POST /api/v1/scenes/stop          # stop test run {id: "..."}
POST /api/v1/scenes/status        # real-time status {id: "..."}
```

### 10.3 DAG Nodes

```
POST /api/v1/scenes/nodes/list    # list nodes {scene_id: "..."}
POST /api/v1/scenes/nodes/add     # add node
POST /api/v1/scenes/nodes/update  # update node
POST /api/v1/scenes/nodes/delete  # remove node
```

### 10.4 DAG Edges

```
POST /api/v1/scenes/edges/add     # add edge
POST /api/v1/scenes/edges/delete  # remove edge
```

### 10.5 Reports

```
POST /api/v1/reports/list         # list reports
POST /api/v1/reports/get          # get report detail
POST /api/v1/reports/html         # download HTML report
```

### 10.6 Plugins

```
POST /api/v1/plugins/list         # list registered plugins
POST /api/v1/plugins/config       # update plugin config
```

### 10.7 Variables

```
POST /api/v1/scenes/variables/list   # list scene variables
POST /api/v1/scenes/variables/set    # set scene variables
```

### 10.8 WebSocket

```
WS /api/v1/ws/run/:id             # real-time metrics stream
```

---

## 11. Web UI

### 11.1 Tech Stack

- Vue 3 + Composition API
- Pinia (state management)
- Vue Router
- Vite (build tool)
- Naive UI (component library)
- @vue-flow/core (DAG visual editor)
- ECharts (real-time charts)
- Axios + WebSocket (API + stream)

### 11.2 Pages

| Page | Description |
|------|-------------|
| Dashboard | Run overview, real-time metrics |
| Scene Editor | DAG visual composition, node/edge config |
| Config | Pool size, timeouts, variables, plugin toggle/config |
| Report | HTML report view, trace chain inspection |

### 11.3 Design Style

Modern internet-style UI, similar to Grafana/Kibana aesthetics.

---

## 12. Configuration

All configurable items support both YAML file and Web UI configuration.

### 12.1 Configuration Items

| Category | Items |
|----------|-------|
| Goroutine Pool | pool_size, run_mode, duration, count |
| Timeout | global_timeout, scene_timeout, node_timeout (per node) |
| Timer | type (ticker/thinktime/random), interval, min, max |
| Variables | global_vars, scene_vars, api_vars |
| Plugins | enabled, config per plugin |
| Rate Limiter | global_qps, url_limits |
| Crypto | algorithm, key, mode |
| Report | format, prometheus_ip, prometheus_port |
| Logger | format (text/json), level, output |
| Database | dialect, dsn, max_connections |

### 12.2 YAML Example

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

## 13. TDD Strategy

### 13.1 Test Pyramid

- **70% Unit Tests** — interface mock implementations, table-driven tests
- **20% Integration Tests** — module interaction (DAG+Pool+Var)
- **10% E2E Tests** — full scenario HTTP testing

### 13.2 Testing Stack

| Tool | Purpose |
|------|---------|
| testing | stdlib, table-driven tests |
| testify/assert | fluent assertions |
| testify/mock | interface mocking |
| testcontainers | MySQL/PG integration tests |
| httptest | HTTP handler testing |
| go test -race | race condition detection |

### 13.3 Coverage Target

≥ 80% line coverage

### 13.4 TDD Workflow

1. Red — write failing test first
2. Green — minimum implementation to pass
3. Refactor — refactor while tests stay green
4. Coverage — verify coverage target met

---

## 14. Git Flow & Commit Convention

### 14.1 Branch Strategy

| Branch | Purpose |
|--------|---------|
| `main` | Production, only merge from release |
| `develop` | Development mainline, daily integration |
| `feature/*` | Feature branches, merge back to develop |
| `hotfix/*` | Emergency fixes, branch from main |

### 14.2 Commit Message Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer: BREAKING CHANGE | Closes #xxx]
```

### 14.3 Types

| Type | Description |
|------|-------------|
| feat | New feature |
| fix | Bug fix |
| refactor | Code refactor (no feature/fix) |
| test | Add/update tests |
| docs | Documentation |
| perf | Performance improvement |
| chore | Build/config/tooling |
| style | Formatting, no logic change |

### 14.4 Scopes

dag, pool, variable, lifecycle, timer, plugin, generator, protocol, api, store, logger, trace, web, config, crypto, report

### 14.5 Milestone Commit Strategy

Each key module must pass all tests before committing:

1. **Interface Definition** — `type(scope): define xxx interface` → Test: mock impl + interface compliance
2. **Core Implementation** — `feat(scope): implement xxx` → Test: table-driven + edge cases
3. **Integration** — `feat(scope): integrate xxx with yyy` → Test: integration test
4. **Release** — `chore(release): bump v0.x.0` → All tests pass + coverage target met

---

## 15. Code Standards

- All code comments in English
- Documentation in both Chinese and English (separate files)
- License: GNU AGPL v3
- Design before development for key features
- Extension guides documented (e.g., "How to add a new protocol")

---

## 16. Skills to Use

| Skill | Purpose |
|-------|---------|
| golang-pro | Go development standards, concurrency patterns |
| sql-pro | Database schema design, query optimization |
| vue-expert | Vue 3 + Composition API development |
| vue-expert-js | Vue 3 JavaScript composables |
| api-designer | REST API design and specification |
| frontend-design | Web UI design and implementation |
