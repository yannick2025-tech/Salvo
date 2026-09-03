# Salvo Implementation Plan

**Date:** 2026-05-01  
**Status:** In Progress  
**Repository:** https://github.com/yannick2025-tech/Salvo

---

## Phase Overview

| Phase | Module | Dependencies | Status |
|-------|--------|-------------|--------|
| P0 | Project Skeleton + Infrastructure | None | Pending |
| P1 | Core Engine | P0 | Pending |
| P2 | Protocol Layer | P1 | Pending |
| P2.5 | Mock HTTP Server | P2 | Pending |
| P3 | Plugin System | P2 | Pending |
| P4 | Parameter Generator | P3 | Pending |
| P5 | Data Storage | P0 | Pending |
| P6 | REST API | P1+P5 | Pending |
| P7 | Trace System | P0+P1 | Pending |
| P8 | Scene Runner | P1-P4 | Pending |
| P9 | Web UI | P6 | Pending |
| P10 | Integration & Release | All | Pending |

---

## P0: Project Skeleton + Infrastructure

**Goal:** Directory structure, logging, snowflake ID, configuration loading

### Tasks

- [ ] Create project directory structure per design spec
- [ ] Implement snowflake ID generator (`internal/pkg/snowflake/`)
  - [ ] Write test: generate unique IDs across goroutines
  - [ ] Write test: JSON marshal/unmarshal as string
  - [ ] Implement generator
- [ ] Implement structured logger (`internal/logger/`)
  - [ ] Define `Logger` interface
  - [ ] Write test: text format output
  - [ ] Write test: JSON format output
  - [ ] Write test: trace_id injection
  - [ ] Implement Zap-based logger
- [ ] Implement config loader (`internal/config/`)
  - [ ] Define `Config` struct matching YAML schema
  - [ ] Write test: load from YAML file
  - [ ] Write test: default values
  - [ ] Implement YAML loader
- [ ] Create entry point (`cmd/salvo/main.go`)
- [ ] Milestone commit: `feat(infra): project skeleton with logger, snowflake, config`

### Acceptance Criteria

- `go build ./...` succeeds
- `go test ./...` passes with ≥ 80% coverage
- Logger outputs both text and JSON formats
- Snowflake ID serializes as string in JSON

---

## P1: Core Engine

**Goal:** DAG executor, goroutine pool, variable system, lifecycle, timer

### P1.1: DAG Executor (`internal/core/dag/`)

- [ ] Define `Node`, `Edge`, `DAG` interfaces
- [ ] Write test: add nodes and edges
- [ ] Write test: detect cycle (validation fails)
- [ ] Write test: topological sort order
- [ ] Write test: root nodes identification
- [ ] Implement `dag` package
- [ ] Milestone commit: `type(dag): define DAG interfaces`

- [ ] Write test: sync execution (wait for response)
- [ ] Write test: async execution (fire and forget)
- [ ] Write test: conditional edge (branch on expression)
- [ ] Write test: loop count on node
- [ ] Write test: fan-out (parallel children)
- [ ] Write test: fan-in (wait all parents)
- [ ] Implement DAG executor
- [ ] Milestone commit: `feat(dag): implement DAG executor`

### P1.2: Goroutine Pool (`internal/core/pool/`)

- [ ] Define `Pool`, `Task`, `RunMode` interfaces
- [ ] Write test: submit tasks to fixed pool
- [ ] Write test: pool size limit enforced
- [ ] Write test: duration mode (run for X time then stop)
- [ ] Write test: count mode (run N iterations then stop)
- [ ] Write test: graceful shutdown via context
- [ ] Write test: submit and wait for result
- [ ] Implement goroutine pool
- [ ] Milestone commit: `feat(pool): implement goroutine pool`

### P1.3: Variable System (`internal/core/variable/`)

- [ ] Define `Scope`, `Store` interfaces
- [ ] Write test: set/get in each scope
- [ ] Write test: lookup order (API → Scene → Global)
- [ ] Write test: inner scope overrides outer
- [ ] Write test: resolve `${A.token}` expression
- [ ] Write test: resolve all params in map
- [ ] Implement variable store
- [ ] Milestone commit: `feat(variable): implement variable system`

### P1.4: Lifecycle (`internal/core/lifecycle/`)

- [ ] Define lifecycle hook interfaces
- [ ] Write test: global setup/teardown called once
- [ ] Write test: scene setup/teardown called per scene
- [ ] Write test: setup can set variables
- [ ] Write test: teardown runs even on error
- [ ] Implement lifecycle manager
- [ ] Milestone commit: `feat(lifecycle): implement lifecycle management`

### P1.5: Timer (`internal/core/timer/`)

- [ ] Define `TimerType`, `Timer` interfaces
- [ ] Write test: ticker fires every X seconds
- [ ] Write test: thinktime waits X seconds then fires
- [ ] Write test: random delay in [min, max] range
- [ ] Write test: timer respects context cancellation
- [ ] Implement timers
- [ ] Milestone commit: `feat(timer): implement ticker, thinktime, random timers`

### P1.6: Context Cascade (`internal/core/contextx/`)

- [ ] Write test: global cancel cancels all scenes
- [ ] Write test: scene timeout does not affect global
- [ ] Write test: node timeout does not affect scene
- [ ] Write test: parent cancel propagates to children
- [ ] Implement context cascade helpers
- [ ] Milestone commit: `feat(contextx): implement context timeout cascade`

### Acceptance Criteria

- All core engine tests pass with ≥ 80% coverage
- DAG can express sync/async/conditional/loop/fan-out/fan-in
- Pool enforces fixed size and supports both run modes
- Variables resolve across all three scopes

---

## P2: Protocol Layer

**Goal:** Protocol interface + HTTP implementation

### Tasks

- [ ] Define `Protocol`, `Request`, `Response` interfaces (`internal/protocol/`)
- [ ] Write test: interface compliance
- [ ] Milestone commit: `type(protocol): define Protocol interface`

- [ ] Implement HTTP protocol (`internal/protocol/http/`)
- [ ] Write test: GET request
- [ ] Write test: POST request with JSON body
- [ ] Write test: request with headers
- [ ] Write test: timeout via context
- [ ] Write test: response parsing (status, headers, body, latency)
- [ ] Implement HTTP protocol
- [ ] Milestone commit: `feat(protocol): implement HTTP protocol`

### Acceptance Criteria

- HTTP protocol passes all tests
- Protocol interface is generic enough for future DB/FTP/gRPC

---

## P2.5: Mock HTTP Server

**Goal:** Built-in test server for E2E testing

### Tasks

- [ ] Create `test/mockserver/` package
- [ ] Implement `/api/login` (auth + token)
- [ ] Implement `/api/users` CRUD (create, get, list, update, delete)
- [ ] Implement `/api/orders` (create, list)
- [ ] Implement `/api/upload` (file upload)
- [ ] Implement `/api/delay/:ms` (configurable latency)
- [ ] Implement `/api/status/:code` (arbitrary status code)
- [ ] Implement `/api/echo` (echo request body)
- [ ] Implement `/api/headers` (echo request headers)
- [ ] Implement `/api/encrypt` (encrypted body round-trip)
- [ ] Implement `/api/chunked` (chunked transfer)
- [ ] Implement `/api/redirect/:count` (chain redirects)
- [ ] Implement `/api/error` (random 500/502/503)
- [ ] Add configurable latency and error rate
- [ ] Add CORS support
- [ ] Write test: each endpoint responds correctly
- [ ] Write test: configurable latency works
- [ ] Write test: error rate works
- [ ] Milestone commit: `feat(mockserver): implement mock HTTP server for E2E testing`

### Acceptance Criteria

- All 18 endpoints functional
- Can start via `go test` or standalone binary
- Latency and error rate configurable

---

## P3: Plugin System

**Goal:** Plugin registry, built-in plugins, Lua engine

### P3.1: Plugin Registry (`internal/plugin/`)

- [ ] Define `Plugin`, `HookPoint`, `Registry` interfaces
- [ ] Write test: register plugin with hooks
- [ ] Write test: get plugins by hook point
- [ ] Write test: plugin lifecycle (init → on_request → on_response → shutdown)
- [ ] Implement registry
- [ ] Milestone commit: `type(plugin): define Plugin and Registry interfaces`

### P3.2: RateLimiter Plugin (`internal/plugin/ratelimit/`)

- [ ] Write test: global QPS limit enforced
- [ ] Write test: per-URL QPS limit enforced
- [ ] Write test: token bucket allows burst
- [ ] Implement rate limiter
- [ ] Milestone commit: `feat(ratelimit): implement rate limiter plugin`

### P3.3: Crypto Plugin (`internal/plugin/crypto/`)

- [ ] Write test: MD5 hash
- [ ] Write test: SHA256 hash
- [ ] Write test: AES encrypt/decrypt
- [ ] Write test: DES encrypt/decrypt
- [ ] Write test: RSA encrypt/decrypt
- [ ] Write test: DSA sign/verify
- [ ] Write test: BCRYPT hash/verify
- [ ] Implement all algorithms
- [ ] Milestone commit: `feat(crypto): implement crypto plugin with MD5/SHA256/AES/DES/RSA/DSA/BCRYPT`

### P3.4: Report Plugin (`internal/plugin/report/`)

- [ ] Write test: HTML report generation
- [ ] Write test: Prometheus metrics endpoint
- [ ] Write test: real-time metrics push
- [ ] Implement report plugin
- [ ] Milestone commit: `feat(report): implement HTML and Prometheus report plugins`

### P3.5: Lua Engine (`internal/plugin/lua/`)

- [ ] Write test: load and execute Lua script
- [ ] Write test: Lua plugin can read/write variables
- [ ] Write test: Lua plugin can make HTTP requests
- [ ] Write test: Lua plugin can log
- [ ] Write test: Lua plugin overrides built-in crypto
- [ ] Implement Lua engine with GopherLua
- [ ] Milestone commit: `feat(lua): implement Lua custom plugin engine`

### Acceptance Criteria

- All built-in plugins pass tests
- Lua plugins can access variables, logging, HTTP
- Plugin registry correctly routes hooks

---

## P4: Parameter Generator

**Goal:** JSON Schema parsing, built-in generators, custom Lua generators

### Tasks

- [ ] Define `Schema`, `Generator` interfaces (`internal/generator/`)
- [ ] Implement JSON Schema Draft 7 parser (`internal/generator/schema/`)
- [ ] Write test: parse Swagger/OpenAPI spec
- [ ] Write test: parse JSON Schema Draft 7
- [ ] Milestone commit: `type(generator): define Schema and Generator interfaces`

- [ ] Implement built-in generators (`internal/generator/builtin/`)
- [ ] Write test: RandomString, RegexString, EnumString
- [ ] Write test: UUIDGenerator, EmailGenerator, DateGenerator
- [ ] Write test: RandomFloat, RandomInt, IncrementInt
- [ ] Write test: RandomBool, WeightedBool
- [ ] Write test: ArrayGenerator, ObjectGenerator
- [ ] Milestone commit: `feat(generator): implement built-in parameter generators`

- [ ] Implement custom Lua generators
- [ ] Write test: Lua generator produces values
- [ ] Write test: Lua generator registered alongside built-in
- [ ] Milestone commit: `feat(generator): implement custom Lua generators`

### Acceptance Criteria

- All JSON Schema Draft 7 keywords supported
- Generators produce valid, schema-compliant values
- Lua generators integrate seamlessly

---

## P5: Data Storage

**Goal:** ent ORM, data models, repositories, migrations

### Tasks

- [ ] Initialize ent with MySQL/PostgreSQL/SQLite dialects
- [ ] Define schema: Scene, Node, Edge, Variable, PluginConfig, Report, RunRecord
- [ ] Generate ent code
- [ ] Implement `Model` base with SnowflakeID + soft delete
- [ ] Write test: create and retrieve entity
- [ ] Write test: soft delete filters out deleted records
- [ ] Write test: SnowflakeID JSON serialization as string
- [ ] Implement repository interfaces (`internal/store/repo/`)
- [ ] Write test: CRUD operations via repository
- [ ] Implement database migrations (`internal/store/migration/`)
- [ ] Write test: migrate up and down
- [ ] Milestone commit: `feat(store): implement data storage with ent ORM`

### Acceptance Criteria

- All models use SnowflakeID with string JSON serialization
- Soft delete works across all repositories
- Migrations work for MySQL, PostgreSQL, SQLite

---

## P6: REST API

**Goal:** All-POST API layer, middleware, DTOs

### Tasks

- [ ] Define DTOs for all endpoints (`internal/api/dto/`)
- [ ] Implement scene management handlers (`internal/api/handler/`)
- [ ] Implement scene execution handlers
- [ ] Implement DAG node/edge handlers
- [ ] Implement report handlers
- [ ] Implement plugin handlers
- [ ] Implement variable handlers
- [ ] Implement WebSocket for real-time metrics
- [ ] Write test: each endpoint with httptest
- [ ] Implement middleware (logging, recovery, CORS)
- [ ] Wire routes with chi/echo router
- [ ] Milestone commit: `feat(api): implement REST API layer with all POST endpoints`

### Acceptance Criteria

- All endpoints use POST method
- Request/response DTOs validated
- WebSocket streams real-time metrics

---

## P7: Trace System

**Goal:** Four-level trace, trace ID propagation

### Tasks

- [ ] Define `Span`, `Tracer` interfaces (`internal/trace/`)
- [ ] Write test: start/finish span
- [ ] Write test: parent-child relationship
- [ ] Write test: span from context
- [ ] Write test: inject trace ID into logger
- [ ] Write test: four-level propagation (scene → chain → API → function)
- [ ] Implement tracer
- [ ] Integrate with logger for automatic trace_id injection
- [ ] Milestone commit: `feat(trace): implement four-level trace system`

### Acceptance Criteria

- Trace IDs propagate correctly across all four levels
- Logger automatically includes trace_id from context

---

## P8: Scene Runner

**Goal:** Orchestrate all core modules for end-to-end scene execution

### Tasks

- [ ] Implement `Runner` that orchestrates DAG + Pool + Variable + Lifecycle + Timer + Plugin + Trace
- [ ] Write test: run simple A→B→C chain
- [ ] Write test: run with parameter correlation
- [ ] Write test: run with conditional branching
- [ ] Write test: run with loop
- [ ] Write test: run with fan-out/fan-in
- [ ] Write test: run in duration mode
- [ ] Write test: run in count mode
- [ ] Write test: lifecycle hooks execute in order
- [ ] Write test: plugins intercept requests/responses
- [ ] Write test: trace IDs present in all logs
- [ ] Write test: context timeout cancels execution
- [ ] Milestone commit: `feat(runner): implement scene runner with full integration`

### Acceptance Criteria

- Full DAG execution with all features working together
- E2E test against mock server succeeds
- Trace IDs present in all log entries

---

## P9: Web UI

**Goal:** Vue 3 frontend for scenario configuration and report viewing

### Tasks

- [ ] Initialize Vue 3 + Vite project (`web/`)
- [ ] Set up Pinia stores, Vue Router, Naive UI
- [ ] Implement Dashboard page (run overview, real-time metrics)
- [ ] Implement Scene Editor page (DAG visual editor with @vue-flow/core)
- [ ] Implement Config page (pool, timeout, variables, plugins)
- [ ] Implement Report page (HTML report, trace chain)
- [ ] Wire API calls with Axios
- [ ] Wire WebSocket for real-time metrics
- [ ] Write component tests
- [ ] Milestone commit: `feat(web): implement Vue 3 Web UI`

### Acceptance Criteria

- All four pages functional
- DAG editor supports drag-and-drop node creation
- Real-time metrics update via WebSocket

---

## P10: Integration & Release

**Goal:** Full integration, build scripts, release

### Tasks

- [ ] Write E2E test suite using mock server
- [ ] Write build scripts (`scripts/build.sh`)
- [ ] Write Dockerfile
- [ ] Write docker-compose.yml (Salvo + MySQL + Prometheus)
- [ ] Configuration file templates (`configs/`)
- [ ] Performance benchmark
- [ ] Security audit (no secrets in code)
- [ ] Milestone commit: `chore(release): bump v0.1.0`

### Acceptance Criteria

- Full E2E test suite passes
- Docker build succeeds
- All tests pass, coverage ≥ 80%

---

## Progress Log

| Date | Phase | Commit | Description |
|------|-------|--------|-------------|
| 2026-05-01 | — | c5b3224 | Initialize Salvo project with design specification |
| 2026-05-01 | — | 4d38e81 | Add mock HTTP server section for E2E testing |
