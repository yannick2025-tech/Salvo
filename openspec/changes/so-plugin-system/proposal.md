## Why

Salvo 当前缺少三大能力，无法支撑 Shell 充电业务全链路压测迁移：

1. **无系统函数引擎**：设计文档定义了 `__weightedChoice`、`__Random`、`__oneOf`、`__manOf`、`__snowflakeId` 等系统函数，但均未实现。现有表达式引擎仅支持 `${var}` 变量替换，不支持函数调用、数学运算、嵌套引用。
2. **缺少关键 DAG 节点**：业务流程需要 `while` 循环（轮询充电状态）、`parallel` 并行（首页初始化）、`sub_flow` 子流程（异步派生），当前仅有 http/delay/condition/if-else/group/timer/setup/teardown 8 种节点。
3. **无动态扩展机制**：Shell 登录流程涉及 AES-CBC 加解密 + BCrypt 哈希，这类定制算法无法通过内置函数覆盖所有业务场景。需要 .so 动态加载机制支持用户自定义算法，且不影响 Salvo 通用性。

## What Changes

### 一、系统函数引擎（统一 `__` 前缀）

- 新增表达式引擎 `internal/core/expr/`，支持函数调用 `${__func(args)}`、数学运算 `${a} * ${b} / 100`、嵌套引用 `${__Random(${min}, ${max})}`
- 实现 6 个系统函数：`__weightedChoice`、`__oneOf`、`__manOf`、`__Random`、`__snowflakeId`、`__so`
- 迁移现有 `generator.email` 等生成器为 `__email()` 统一前缀格式
- YAML 配置层 `config_params` / `derived_params` 支持映射到系统函数

### 二、新增 DAG 节点

- **`while` 节点**：循环执行子步骤，支持 `exit_conditions`（退出条件）、`interval_seconds`（轮询间隔）、`max_iterations`（最大迭代次数）、`max_duration_minutes`（最大持续时长）、`fail_after_consecutive`（连续失败阈值）
- **`parallel` 节点**：并行执行多个子步骤，等待全部完成后返回
- **`sub_flow` 节点**：引用另一个场景作为子流程，支持 `async: true` 异步派生
- **`loop` 节点**：已有常量 `NodeTypeLoop` 但未实现，补充执行逻辑（固定次数循环，与 while 的条件循环互补）

### 三、SO 动态插件系统

- **Go plugin 方案**：使用 Go 标准 `plugin` 包加载 .so 文件，用户编写实现 `so.Plugin` 接口的 Go 代码编译为 .so
- **版本管理**：插件支持 `Version()` 方法，同一插件可多版本共存，`Get(name, version)` 默认取最新版本，日志始终打印实际调用的 `name@version`
- **持久化**：.so 文件上传后持久存储到磁盘，Salvo 启动时自动加载所有已启用的插件，用户只需上传一次
- **表达式集成**：通过 `${__so("plugin@version", "op", "arg1", "arg2")}` 在表达式中间件中调用
- **GUI 管理页面**：新增「SO 插件管理」页面（仅管理员可见），支持上传、列表、删除（物理删除）、废弃（逻辑禁用），显示插件名称、版本、支持操作列表、状态
- **菜单重构**：当前「系统设置」页面（修改密码 + 个人信息）改名为「个人设置」，新增「SO 插件管理」菜单项

### 四、条件运算符增强

- 新增 12 个条件运算符：`equals`、`not_equals`、`greater_than`、`greater_than_or_equal`、`less_than`、`less_than_or_equal`、`not_empty`、`empty`、`size_equals`、`size_greater_than`、`size_greater_than_or_equal`、`size_less_than`

## Capabilities

### New Capabilities
- `expr-engine`: 表达式引擎（函数调用、数学运算、嵌套引用、`__` 前缀统一规范）
- `builtin-functions`: 6 个系统函数实现（`__weightedChoice`、`__oneOf`、`__manOf`、`__Random`、`__snowflakeId`、`__so`）
- `while-node`: while 循环节点（退出条件、轮询间隔、最大迭代、连续失败检测）
- `parallel-node`: parallel 并行节点（多步骤并行执行、等待全部完成）
- `sub-flow-node`: sub_flow 子流程节点（引用场景、异步派生）
- `so-plugin-system`: SO 动态插件系统（Go plugin 加载、版本管理、持久化、表达式集成）
- `so-plugin-gui`: SO 插件管理 GUI（上传、列表、删除、废弃、管理员权限）
- `condition-operators`: 条件运算符增强（12 个运算符）

### Modified Capabilities
- `scene-data-integrity`: YAML 导入/导出扩展支持 while/parallel/sub_flow 节点类型和 config_params/derived_params

## Impact

- **后端新增**：`internal/core/expr/`（表达式引擎）、`internal/plugin/so/`（SO 插件系统）、`internal/generator/builtin/`（系统函数）
- **后端修改**：`internal/runner/runner.go`（新增 while/parallel/sub_flow/loop 节点执行逻辑）、`internal/store/model/model.go`（新增 SO 插件模型、节点类型常量）、`internal/api/handler.go`（SO 插件管理 API）
- **前端新增**：`web/app/src/views/plugins/PluginsPage.vue`（SO 插件管理页面）
- **前端修改**：`web/app/src/layouts/MainLayout.vue`（菜单重构）、`web/app/src/views/settings/SettingsPage.vue`（改名为个人设置）、`web/app/src/views/scenes/SceneDetailPage.vue`（新增节点类型配置面板）
- **数据库**：新增 `so_plugins` 表（插件元数据、文件路径、状态、配置）
- **YAML 格式**：扩展 `while`、`parallel`、`sub_flow` 节点类型，`config_params` / `derived_params` 参数生成模式
