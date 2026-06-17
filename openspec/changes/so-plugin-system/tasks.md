## TDD 策略说明

> 遵循 `.knowledge/L1-conventions/tdd-strategy.md`，所有模块按 **红灯 → 绿灯 → 重构** 循环推进。
> - 70% 单元测试（表驱动 + testify/assert + mock）
> - 20% 集成测试（模块间交互）
> - 10% 端到端测试（完整 HTTP 场景）
> - 覆盖率目标：≥ 80% 行覆盖率
> - 并发安全测试：`go test -race`
> - 概率函数：10000 样本统计验证

---

## 1. Expression Engine (TDD)

### 红灯：先写失败测试

- [x] 1.1 创建 `internal/core/expr/engine_test.go`，表驱动测试覆盖以下场景：
  - 变量引用：`${name}` → 值替换
  - 函数调用：`${__random(60, 600)}` → 函数执行
  - 数学运算：`${a} * ${b} / 100` → 算术求值
  - 嵌套引用：`${__random(${min}, ${max})}` → 内层先解析
  - 混合表达式：`"用户:${name}, ID:${__snowflakeId()}"`
  - 无表达式：纯文本原样返回
  - 空字符串：返回空
  - 循环引用检测：`${a}` → `${b}` → `${a}`，返回错误
  - 最大深度超限：11 层嵌套，返回错误
  - 未注册函数：`${__unknown()}`，原样保留并记录警告
  - 函数参数为空：`${__random()}`，返回错误
- [x] 1.2 创建 `internal/core/expr/math_eval_test.go`，表驱动测试覆盖：
  - 简单运算：`60 * 50 / 100` → `30`
  - 括号：`(10 + 20) * 3` → `90`
  - 浮点运算：`1.5 * 2` → `3`
  - 除零：`10 / 0` → 返回错误
  - 非法字符：`10 # 5` → 返回错误
  - 空表达式：`` → 返回空
  - 负数：`-5 + 10` → `5`
- [x] 1.3 创建 `internal/core/expr/registry_test.go`，测试 FunctionRegistry：
  - Register + Get 正常流程
  - 重复注册返回错误
  - Get 未注册函数返回 false
  - List 返回所有已注册函数名

### 绿灯：最小实现使测试通过

- [x] 1.4 创建 `internal/core/expr/registry.go`：实现 `FunctionRegistry`（Register/Get/List）
- [x] 1.5 创建 `internal/core/expr/engine.go`：实现 `Resolve(input, variables, registry)` — 大括号计数解析嵌套 `${...}`，分发到变量替换/函数调用/数学运算
- [x] 1.6 创建 `internal/core/expr/math_eval.go`：实现受限算术表达式求值（+ - * / 括号，无变量赋值）
- [x] 1.7 实现嵌套递归解析（max 10 depth），循环引用检测

### 重构 + 覆盖率

- [x] 1.8 运行 `go test -race -cover ./internal/core/expr/`，验证覆盖率 ≥ 80% (86.6%)
- [ ] 1.9 迁移现有 `generator.email` → `__email()` 格式，更新调用方
- [ ] 1.10 集成到 runner.go：HTTP 节点的 URL/Body/Header 字段使用新引擎解析

---

## 2. Builtin System Functions (TDD — 重点覆盖)

### 红灯：`__weightedChoice` 测试

- [x] 2.1 创建 `internal/generator/builtin/weighted_choice_test.go`，表驱动测试：
  - 二选一等概率：`"1=50,0=50"` → 10000 次采样，1 和 0 各约 50%（±5%）
  - 多选一加权：`"A=40,B=30,C=20,D=10"` → 各约 40/30/20/10%（±5%）
  - 权重和 < 100 归一化：`"A=40,B=30,C=20"`（和=90）→ A≈44.4%, B≈33.3%, C≈22.2%
  - 权重和 > 100 归一化：`"A=50,B=50,C=50"`（和=150）→ 各约 33.3%
  - 单选项：`"A=100"` → 始终返回 "A"
  - 零权重过滤：`"A=0,B=50"` → 过滤 A，B=100%
  - 负权重过滤：`"A=-1,B=50"` → 过滤 A
  - 空字符串：`` → 返回错误
  - 格式错误：`"A,B,C"`（无等号）→ 返回错误
  - 权重非数字：`"A=xx"` → 返回错误
  - 单 key 多等号：`"A=50=extra"` → 返回错误
  - 并发安全：100 goroutine 同时调用，`go test -race` 通过

### 红灯：`__oneOf` 测试

- [x] 2.2 创建 `internal/generator/builtin/one_of_test.go`，表驱动测试：
  - 三选一等概率：`("A","B","C")` → 10000 次采样，各约 33.3%（±5%）
  - 单参数：`("A")` → 始终返回 "A"
  - 两参数：`("A","B")` → 各约 50%（±5%）
  - 空参数：`()` → 返回错误
  - 含特殊字符：`("hello world","foo,bar")` → 正确返回
  - 并发安全：100 goroutine 同时调用

### 红灯：`__manOf` 测试

- [x] 2.3 创建 `internal/generator/builtin/man_of_test.go`，表驱动测试：
  - 七选子集：`(1,2,3,4,5,6,7)` → 10000 次采样，子集大小 1-7，每个元素出现概率约 50%
  - 单参数：`(1)` → 始终返回 "1"
  - 两参数：`(1,2)` → 子集大小 1-2
  - 空参数：`()` → 返回错误
  - 返回格式：逗号分隔，无空格
  - 并发安全：100 goroutine 同时调用

### 红灯：`__random` 测试

- [x] 2.4 创建 `internal/generator/builtin/random_test.go`，表驱动测试：
  - 整数范围：`(60, 600)` → 10000 次采样，结果 ∈ [60, 600]，均值约 330（±30）
  - 整数边界包含：`(1, 1)` → 始终返回 "1"
  - 整数边界包含：`(0, 100)` → 结果 ∈ [0, 100]
  - 整数 min > max：`(600, 60)` → 返回 min（60）
  - 浮点范围：`(1.5, 9.5, 2)` → 结果 ∈ [1.50, 9.50]，保留 2 位小数
  - 浮点 scale=0：`(1.0, 10.0, 0)` → 整数结果
  - 浮点 scale=4：`(0.0, 1.0, 4)` → 4 位小数
  - 浮点 min > max：`(9.5, 1.5, 2)` → 返回 min（1.50）
  - 非数字参数：`("abc", "def")` → 返回错误
  - 参数个数错误：`(60)` → 返回错误（需 2 或 3 个参数）
  - 参数个数错误：`(60, 600, 2, 3)` → 返回错误
  - 并发安全：100 goroutine 同时调用

### 红灯：`__snowflakeId` 测试

- [x] 2.5 创建 `internal/generator/builtin/snowflake_test.go`，表驱动测试：
  - 格式验证：返回合法数字字符串
  - 唯一性：10000 次调用，无重复
  - 单调递增：连续调用，ID 递增
  - 并发安全：100 goroutine 各生成 100 个 ID，无重复，`go test -race` 通过

### 绿灯：实现所有函数

- [x] 2.6 实现 `__weightedChoice` in `internal/generator/builtin/weighted_choice.go`
- [x] 2.7 实现 `__oneOf` in `internal/generator/builtin/one_of.go`
- [x] 2.8 实现 `__manOf` in `internal/generator/builtin/man_of.go`
- [x] 2.9 实现 `__random` in `internal/generator/builtin/random.go`（整数 + 浮点双模式）
- [x] 2.10 实现 `__snowflakeId` in `internal/generator/builtin/snowflake.go`（复用 `internal/pkg/snowflake`）
- [x] 2.11 在 `register.go` 中注册所有函数到 FunctionRegistry

### 重构 + 覆盖率

- [x] 2.12 运行 `go test -race -cover ./internal/generator/builtin/`，验证覆盖率 ≥ 80% (89.8%)
- [x] 2.13 验证概率分布：10000 样本统计，各函数概率偏差 ≤ 5%

---

## 3. Condition Operators (TDD)

### 红灯：先写失败测试

- [x] 3.1 创建 `internal/core/expr/evaluator_test.go`，表驱动测试 12 个运算符：
  - `equals`：字符串相等 `"4" == "4"` → true；数字相等 `5 == 5` → true；不等 `"3" == "4"` → false
  - `not_equals`：字符串不等 `"COMPLETED" != "PENDING"` → true；相等 → false
  - `greater_than`：`5 > 0` → true；`0 > 5` → false；`5 > 5` → false
  - `greater_than_or_equal`：`5 >= 5` → true；`4 >= 5` → false
  - `less_than`：`3 < 10` → true；`10 < 3` → false；`3 < 3` → false
  - `less_than_or_equal`：`3 <= 3` → true；`4 <= 3` → false
  - `not_empty`：变量有值 → true；变量为空字符串 → false；变量不存在 → false；变量为 nil → false
  - `empty`：变量为空字符串 → true；变量不存在 → true；变量有值 → false
  - `size_equals`：数组长度 1 == "1" → true；长度 2 == "1" → false
  - `size_greater_than`：数组长度 3 > "1" → true；长度 1 > "1" → false
  - `size_greater_than_or_equal`：数组长度 1 >= "1" → true；长度 0 >= "1" → false
  - `size_less_than`：数组长度 1 < "5" → true；长度 5 < "5" → false
  - 边界：变量不存在时所有运算符返回 false（除 `empty` 返回 true）
  - 边界：value 为空字符串时 `equals` 比较空字符串
  - 边界：非数字变量做 `greater_than` → 返回 false（不 panic）
  - 边界：非数组变量做 `size_equals` → 返回 false

### 绿灯：实现

- [x] 3.2 创建 `internal/core/expr/evaluator.go`：实现 `EvaluateCondition(variable, operator, value, variables) bool`
- [x] 3.3 集成到 while 退出条件、if-else 分支、步骤条件、DAG 条件边
  - `evaluateExpression` → 使用 `EvaluateConditionExpr` 替代 truthy 检查
  - `executeCondition` → 使用 `EvaluateConditionExpr` 替代 `resolveWithVariables` + truthy 检查
  - `evalCondition` → 使用 `EvaluateConditionExpr`，保留 `__if_true__` / `__if_false__` 特殊处理
  - 新增 `EvaluateConditionExpr(expr, variables) bool` 解析 `${var} operator value` 格式
  - 新增 `parseConditionExpr(expr)` 正则解析器，支持符号运算符和关键字运算符

### 重构 + 覆盖率

- [x] 3.4 运行 `go test -race -cover ./internal/core/expr/`，验证覆盖率 ≥ 80% (83.6%)

---

## 4. While Loop Node (TDD)

### 红灯：先写失败测试

- [x] 4.1 创建 `internal/runner/while_node_test.go`，测试：
  - 退出条件满足：exit_condition `status == "4"`，第 3 次迭代 status 变为 "4" → 循环退出，返回成功
  - 退出条件不满足：exit_condition 永远不满足 + max_iterations=5 → 5 次后强制退出，返回错误
  - max_duration_minutes 超时：设置 0.01 分钟（0.6 秒），interval=0.2s → 超时退出
  - fail_after_consecutive：子步骤连续失败 3 次 → 节点失败，返回 fail_message
  - fail_after_consecutive 恢复：失败 2 次后成功 → 计数器重置
  - interval_seconds 等待：interval=1 → 每次迭代间等待 1 秒
  - timed_trigger once=true：第 1 次迭代触发，后续迭代不触发
  - think_time：子步骤间随机延迟
  - 子步骤变量提取：step1 提取 status，exit_condition 使用 status
  - 空步骤列表：steps=[] → 返回错误
  - 无 exit_conditions 且无 max_iterations → 返回错误（无限循环保护）

### 绿灯：实现

- [x] 4.2 添加 `NodeTypeWhile = "while"` 到 `internal/store/model/model.go`
- [x] 4.3 创建 `internal/runner/while_node.go`：实现 `executeWhile()` 循环执行 steps → 检查 exit_conditions → 等待 interval → 重复，包括子步骤 HTTP 请求、变量提取、JSON Path 解析
- [x] 4.4 实现 max_iterations / max_duration_minutes / fail_after_consecutive / timed_trigger / think_time / retry / 子步骤条件

### 重构 + 覆盖率

- [x] 4.5 运行 `go test -race -cover -run While ./internal/runner/`，全部 25 个测试通过，新增 while_node.go + while_node_test.go 覆盖 exit_conditions、max_iterations、上下文取消、无限循环保护、空步骤、子步骤条件、连续失败、think_time、JSON Path 解析、Extract Entry 解析、HTTP 请求构建、变量提取、多条件退出、重试、时间触发等场景

---

## 5. Parallel Node (TDD)

### 红灯：先写失败测试

- [x] 5.1 创建 `internal/runner/parallel_node_test.go`，测试：
  - 全部成功：3 个子步骤并行执行，全部成功 → 节点成功
  - 部分失败：3 个子步骤，其中一个失败 → 节点返回错误，但所有步骤仍执行
  - 变量隔离：step A 提取 varA，step B 提取 varB → 并行执行时互不可见
  - 变量合并：并行完成后，varA 和 varB 都在父作用域可用
  - 变量冲突：step A 和 step B 都提取 `token` → 最后完成的覆盖（last-write-wins）
  - 并发安全：50 个子步骤并行，`go test -race` 通过
  - 空步骤列表：steps=[] → 返回成功（无操作）
  - 子步骤条件：条件不满足时跳过执行
  - Nil input：input=nil → 正常运行
  - 上下文取消：已取消的 context → 返回错误
  - 初始变量合并：初始变量与提取变量合并

### 绿灯：实现

- [x] 5.2 添加 `NodeTypeParallel = "parallel"` 到 model.go
- [x] 5.3 创建 `internal/runner/parallel_node.go`：实现 `executeParallel()` — goroutine + WaitGroup，变量隔离 + 合并，子步骤 HTTP 请求/提取/think_time

### 重构 + 覆盖率

- [x] 5.4 运行 `go test -race -cover -run Parallel ./internal/runner/`，9 个测试全部通过，覆盖全部成功/部分失败/空步骤/变量隔离/变量冲突/并发安全/条件跳过/已取消上下文/初始变量合并等场景

---

## 6. Sub-flow Node (TDD)

### 红灯：先写失败测试

- [x] 6.1 创建 `internal/runner/sub_flow_node_test.go`，测试：
  - 同步执行：scene_id 引用子场景，async=false → 等待子场景完成
  - 异步执行：async=true → 立即返回，子场景后台执行
  - 变量合并（同步）：子场景提取 subToken → 父作用域可用
  - 变量不合并（异步）：async 返回后父作用域无子场景变量
  - 深度限制：嵌套 5 层 → 正常；嵌套 6 层 → 返回错误（depth >= 5 拒绝）
  - 循环引用检测：A→B→A → 返回错误
  - 场景不存在：scene_id 无效 → runner 返回错误
  - 空 scene_id → 返回错误
  - subFlowRunner 为 nil → 返回错误
  - 上下文取消 → 返回错误
  - Nil input → 正常运行
  - 异步不阻塞：慢子场景 200ms，async=true 在 100ms 内返回

### 绿灯：实现

- [x] 6.2 添加 `NodeTypeSubFlow = "sub_flow"` 到 model.go
- [x] 6.3 创建 `internal/runner/sub_flow_node.go`：实现 `executeSubFlow()` — 通过 subFlowRunner 函数加载子场景，支持 sync/async 模式，通过 context.Value 跟踪深度 (subFlowDepthKey) 和访问链 (subFlowVisitedKey)，深度限制 5 层，循环引用检测

### 重构 + 覆盖率

- [x] 6.4 运行 `go test -race -cover -run SubFlow ./internal/runner/`，11 个测试全部通过，覆盖 sync/async/变量合并/深度限制/循环检测/场景不存在/空 ID/nil runner/上下文取消/nil input/异步不阻塞

---

## 7. Loop Node (TDD)

### 红灯：先写失败测试

- [x] 7.1 创建 `internal/runner/loop_node_test.go`，测试：
  - 固定循环 3 次：loop_count=3 → 子步骤执行 3 次
  - loop_count=0：不执行
  - loop_count=1：执行 1 次
  - 变量累积：每次迭代提取的变量在下次迭代可用（iterationNum=3）
  - 与 while 区别：loop 是固定次数，while 是条件退出
  - 空步骤列表：steps=[] → 不执行
  - 上下文取消 → 返回错误
  - 子步骤条件：条件不满足时跳过执行
  - Nil input：正常运行
  - Think time：每次迭代的 think_time 生效

### 绿灯：实现

- [x] 7.2 创建 `internal/runner/loop_node.go`：实现 `executeLoop()` — 固定次数循环执行子步骤，通过 mergedVars 变量累积跨迭代传递变量，复用 buildStepHTTPRequest/extractVarsFromResponse，支持 think_time、条件跳过、上下文取消

### 重构 + 覆盖率

- [x] 7.3 运行 `go test -race -cover -run Loop ./internal/runner/`，10 个测试全部通过，覆盖固定次数/零次/一次/变量累积/空步骤/上下文取消/条件跳过/nil input/think time

---

## 8. SO Plugin System (TDD)

### 红灯：先写失败测试

- [x] 8.1 创建 `internal/plugin/so/loader_test.go`，测试：
  - Get 最新版本：加载 v1.0.0 和 v1.1.0 → Get("name", "") 返回 v1.1.0
  - Get 指定版本：Get("name", "1.0.0") 返回 v1.0.0
  - Get 不存在：Get("unknown", "") → 返回 false
  - List 排序：按 name 升序，version 降序
  - 并发安全：100 goroutine 同时 Get，`go test -race` 通过
- [x] 8.2 创建 `internal/plugin/so/adapter_test.go`，测试 `__so` 表达式解析：
  - 最新版本调用：`${__so("plugin", "op", "arg")}` → 调用最新版本
  - 指定版本调用：`${__so("plugin@1.0.0", "op", "arg")}` → 调用 v1.0.0
  - 插件不存在：返回错误
  - Call 失败：返回错误
  - 空参数：`${__so("plugin", "op")}` → Call("op") 无 args
  - 带引号参数：`${__so("plugin", "op", "arg with space")}` → 正确解析
  - 参数不足 2 个：返回 "requires at least 2 arguments"

### 绿灯：实现

- [x] 8.3 创建 `internal/plugin/so/contract.go`：Plugin 接口 + Factory 类型
- [x] 8.4 创建 `internal/plugin/so/loader.go`：Loader 实现（Load/Get/List，版本索引，日志）
- [x] 8.5 创建 `internal/plugin/so/adapter.go`：SOFunction 表达式适配器
- [x] 8.6 创建 `internal/store/model/so_plugin.go`：SOPlugin 模型
- [x] 8.7 创建 `so_plugins` 表 migration
- [x] 8.8 实现 `SOPluginRepo`（Create/GetByID/List/UpdateStatus/UpdateConfig/Delete）
- [x] 8.9 实现 SO 插件 API handlers（upload/list/get/status/config/delete，admin-only）
- [x] 8.10 实现启动自动加载：查询 status=enabled，调用 Loader.Load()
- [x] 8.11 注册 `__so` 到 builtin registry

### 重构 + 覆盖率

- [x] 8.12 运行 `go test -race -cover ./internal/plugin/so/`，验证覆盖率 ≥ 80%
- [ ] 8.13 创建示例 SO 插件 `plugins/shell-aes/main.go`（AES-CBC 加解密，匹配 login.py）

---

## 9. Frontend: SO Plugin Management Page

- [ ] 9.1 创建 `web/app/src/views/plugins/PluginsPage.vue`：表格布局（Name, Version, Ops, Status, CreatedAt, Actions）
- [ ] 9.2 实现文件上传：POST /api/v1/plugins/so/upload（multipart），loading 状态，错误处理
- [ ] 9.3 实现插件列表：GET /api/v1/plugins/so/，按 name+version 排序
- [ ] 9.4 实现 Disable/Enable：PUT /:id/status，确认对话框，toast 通知
- [ ] 9.5 实现 Delete：DELETE /:id，确认对话框（"不可恢复"），toast 通知
- [ ] 9.6 实现 Config 编辑器：modal + JSON textarea，PUT /:id/config
- [ ] 9.7 样式与 UsersPage.vue 保持一致（card/table/modal/badge）

---

## 10. Frontend: Menu & Settings Restructure

- [ ] 10.1 `MainLayout.vue`：设置菜单标签 "系统设置" → "个人设置"
- [ ] 10.2 新增菜单项：`{ path: '/plugins', label: 'SO 插件管理', icon: PluginIcon, permission: 'plugins:read' }`
- [ ] 10.3 创建 PluginIcon SVG 组件（拼图/包图标）
- [ ] 10.4 添加路由 `/plugins` → PluginsPage.vue
- [ ] 10.5 `SettingsPage.vue`：页面标题 "系统设置" → "个人设置"
- [ ] 10.6 验证 `canAccess(['plugins:read'])` 仅 admin 返回 true

---

## 11. Frontend: DAG Node Editor Extensions

- [ ] 11.1 SceneDetailPage.vue 节点创建菜单新增 while/parallel/sub_flow/loop 类型
- [ ] 11.2 while 节点配置面板：exit_conditions 编辑器（variable/operator/value 行）、interval_seconds、max_iterations、max_duration_minutes、fail_after_consecutive、fail_message、steps 编辑器
- [ ] 11.3 parallel 节点配置面板：steps 编辑器（复用 HTTP step 表单）
- [ ] 11.4 sub_flow 节点配置面板：场景选择下拉框、async 复选框
- [ ] 11.5 loop 节点配置面板：loop_count 输入、steps 编辑器
- [ ] 11.6 DagFlow.vue 渲染新节点类型（不同图标和标签）

---

## 12. Backend: YAML Import/Export Extension

- [ ] 12.1 扩展 `yamlNode` 支持 while/parallel/sub_flow/loop 类型及配置字段
- [ ] 12.2 ImportYAML：解析 while/parallel/sub_flow/loop 节点，创建 Node 记录
- [ ] 12.3 YAML 导出：序列化 while/parallel/sub_flow/loop 节点完整配置
- [ ] 12.4 支持 `config_params` 和 `derived_params` 段落（映射到系统函数）
- [ ] 12.5 支持 `think_time`、`retry`、`condition`、`timed_trigger` 字段

---

## 13. Integration & E2E Testing

### 集成测试（20%）

- [ ] 13.1 集成测试：表达式引擎 + runner 集成 — `${__random(60, 600)}` 在 HTTP URL 中正确解析
- [ ] 13.2 集成测试：表达式引擎 + SO 插件 — `${__so("shell-aes", "encrypt", "data")}` 调用已加载插件
- [ ] 13.3 集成测试：while 节点 + 条件运算符 — 轮询直到 exit_condition 满足
- [ ] 13.4 集成测试：parallel 节点 + 变量系统 — 并行提取变量并合并
- [ ] 13.5 集成测试：SO 插件上传 API + Loader + 表达式调用 全链路

### 端到端测试（10%）

- [ ] 13.6 E2E：完整 Shell 登录流程 — get-app-salt → AES 解密 → BCrypt 哈希 → username-login（使用 SO 插件）
- [ ] 13.7 E2E：card.yaml 场景导入 + 执行 — 验证 while 轮询充电状态、parallel 首页初始化
- [ ] 13.8 E2E：SO 插件管理全流程 — 上传 → 列表 → 配置 → 废弃 → 重启 → 验证未加载 → 删除
- [ ] 13.9 E2E：管理员/普通用户权限 — admin 可访问 /plugins，普通用户 403
- [ ] 13.10 E2E：YAML 导入导出往返 — while/parallel/sub_flow 节点完整保留

### 覆盖率验证

- [ ] 13.11 运行 `go test -race -cover ./internal/...`，验证整体覆盖率 ≥ 80%
- [ ] 13.12 运行 `go test -race ./...`，确保无竞态条件
