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
