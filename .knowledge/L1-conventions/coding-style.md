---
layer: L1
maturity: proven
last-verified: 2026-05-25
source: .trae/rules/project_rules.md + .trae/skills/gater-conventions/SKILL.md
tags: [go, vue, typescript, code-style, naming, comments, error-handling]
---

# 代码风格规范

> Go + Vue/TypeScript 代码编写必须遵循的样式约定。

## Go 代码风格

### Package Comments (包注释)
- **每个 Go 包必须有包注释**，格式为 `// Package xxx provides/does ...`
- 包注释只需出现在包内一个源文件顶部（通常放在主文件或 `doc.go`)
- 新建 `.go` 文件时，如果该文件是包内第一个文件，必须添加包注释
- 示例：`// Package pool provides a fixed-size goroutine pool for driving test`
- 参考：[Effective Go - Package comments](https://go.dev/doc/effective_go#package-comments)

### Exported Symbols (导出符号注释)
- 所有导出的函数、类型、变量、常量必须有文档注释
- 注释以符号名开头：`// WaitTimeTracker records task wait durations...`
- 注释应为完整句子，描述用途而非实现细节

### Error Handling (错误处理)
- 不使用 panic，使用 error 返回值
- 错误信息小写开头，不加句号
- 使用 `fmt.Errorf("context: %w", err)` 包装错误

### Function Style (函数风格)
- 每个函数后面保留一个空行
- 禁止使用单行函数（函数体和签名不能在同一行）

### 命名规范
- 包名：小写单个单词，不使用下划线或驼峰
- 导出函数/变量：大写字母开头（PascalCase）
- 未导出函数/变量：小写字母开头（camelCase）
- 接口名：以 `-er` 后缀表示行为（如 `Reader`, `Writer`）
- 常量：使用 camelCase，不使用全大写

## Vue / TypeScript 代码风格

### 组件规范
- 组件文件名：PascalCase（如 `PriceTable.vue`）
- 组件名称：PascalCase
- Props / Emits：使用 `defineProps` / `defineEmits`，添加 TypeScript 类型

### 变量与函数
- 变量/函数：camelCase
- 常量：UPPER_SNAKE_CASE
- 使用 Composition API + `<script setup>`

### 模板规范
- 属性顺序：`v-if` / `v-for` → `v-model` → 事件绑定 → 其他属性
- CSS 使用 `<style scoped>`，类名使用 kebab-case

## 通用规则

- 缩进：Go 用 Tab，Vue/TS 用 2 空格
- 行宽：软限制 120 字符
- 文件末尾保留一个空行
- 不提交 IDE 配置文件（`.idea/`, `.vscode/` 等）

## 日志规范 (Logging Conventions)

### 日志级别

- **Info**：关键里程碑（start/stop/completed），节点执行成功，场景生命周期事件
- **Warn**：非致命异常（数据源加载失败，成功率低于阈值），意料之中的边界情况
- **Error**：Panic recovery，节点执行失败，初始化失败（buildDAG/buildScope/setup），系统错误

### Golang 日志模式

#### 1. 优先使用 `logger.WithContext(ctx)` 而非手动拼字段

```go
// ✅ 正确：使用 WithContext 从 context 提取 trace_id/chain_id/node_id/scene_id
runCtx := logger.ContextWithTraceID(r.ctx, traceID)
runCtx = logger.ContextWithSceneID(runCtx, r.cfg.SceneID.String())
runLog := r.log.WithContext(runCtx)

// ❌ 避免手动构造字段（除非 context 无法提供）
runLog := r.log.With(logger.F("trace_id", traceID), logger.F("scene_id", sceneID))
```

#### 2. 结构化字段使用 `logger.F()`，禁止字符串拼接

```go
// ✅ 正确
runLog.Info("scene run started", logger.F("workers", r.cfg.Workers))

// ❌ 错误
runLog.Info(fmt.Sprintf("scene run started: %d workers", r.cfg.Workers))
```

#### 3. Goroutine panic recovery 统一使用 `safeGo`

```go
// ✅ 正确
safeGo(ctx, log, "worker-name", func() {
    // goroutine logic
})

// ❌ 避免自己写 defer/recover
go func() {
    defer func() { ... }()
    // ...
}()
```

#### 4. 节点执行日志范式

```go
nodeLog.Info("node execution started")
out, err := n.doSomething(ctx, input, nodeLog)
if err != nil {
    nodeLog.Error("node execution failed", logger.F("error", err))
} else {
    nodeLog.Info("node execution completed")
}
```

#### 5. 错误传播与失败记录

- 使用 `r.setError(err)` 存储首个错误（通过 `atomic.Value`）
- 在 buildDAG/buildScope/setup 失败时调用 `r.createFailedRunRecord()` 创建失败记录
- API Handler 层记录详细错误日志后再返回给客户端

### Context 注入的字段

| 字段名 | Context Key | 注入时机 |
|--------|-------------|---------|
| `trace_id` | `logger.traceIDKey` | Runner.Run() 开始 |
| `scene_id` | `logger.sceneIDKey` | Runner.Run() / execute() 开始 |
| `chain_id` | `logger.chainIDKey` | 每轮迭代开始 |
| `node_id` | `logger.nodeIDKey` | sceneNode.Execute() 开始 |
