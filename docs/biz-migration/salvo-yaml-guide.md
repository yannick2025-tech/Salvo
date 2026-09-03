# Salvo 场景 YAML 导入指南

> 本文档详细说明通过 `POST /api/v1/scenes/import` 导入场景 YAML 时，Salvo 支持的节点类型、变量、内置函数、SO 插件的用法与示例。
>
> 代码参考：
> - 节点类型定义：[model.go#L59-L71]($PROJECT_HOME/salvo/internal/store/model/model.go#L59-L71)
> - 节点执行分发：[runner.go#L1330-L1351]($PROJECT_HOME/salvo/internal/runner/runner.go#L1330-L1351)
> - YAML 导入解析：[handler.go#L56-L116]($PROJECT_HOME/salvo/internal/api/handler.go#L56-L116)
> - 表达式引擎：[engine.go]($PROJECT_HOME/salvo/internal/core/expr/engine.go)

---

## 目录

- [Salvo 场景 YAML 导入指南](#salvo-场景-yaml-导入指南)
  - [目录](#目录)
  - [1. YAML 顶层结构](#1-yaml-顶层结构)
  - [2. DAG 执行模型](#2-dag-执行模型)
    - [2.1 拓扑排序与顺序执行](#21-拓扑排序与顺序执行)
    - [2.2 独立分支并行](#22-独立分支并行)
    - [2.3 边条件控制](#23-边条件控制)
  - [3. 场景变量与数据源](#3-场景变量与数据源)
    - [3.1 variables 场景变量](#31-variables-场景变量)
    - [3.2 config\_params 配置参数](#32-config_params-配置参数)
    - [3.3 derived\_params 派生参数](#33-derived_params-派生参数)
    - [3.4 data\_sources CSV 数据源](#34-data_sources-csv-数据源)
      - [2.4.1 YAML 与 CSV 数据源共存](#241-yaml-与-csv-数据源共存)
  - [4. 节点类型详解](#4-节点类型详解)
    - [4.1 http HTTP 请求节点](#41-http-http-请求节点)
    - [4.2 setup / teardown 生命周期节点](#42-setup--teardown-生命周期节点)
    - [4.3 generator 生成器节点](#43-generator-生成器节点)
    - [4.4 delay 延迟节点](#44-delay-延迟节点)
    - [4.5 timer 定时器节点](#45-timer-定时器节点)
    - [4.6 condition 条件判断节点](#46-condition-条件判断节点)
    - [4.7 if-else 条件分支节点](#47-if-else-条件分支节点)
    - [4.8 group 分组节点](#48-group-分组节点)
    - [4.9 while 循环节点](#49-while-循环节点)
    - [4.10 loop 循环节点](#410-loop-循环节点)
    - [4.11 parallel 并行节点](#411-parallel-并行节点)
    - [4.12 sub\_flow 子流程节点](#412-sub_flow-子流程节点)
  - [5. 内置系统函数](#5-内置系统函数)
    - [5.1 \_\_random 随机数](#51-__random-随机数)
    - [5.2 \_\_weightedChoice 加权选择](#52-__weightedchoice-加权选择)
    - [5.3 \_\_oneOf 等概率选择](#53-__oneof-等概率选择)
    - [5.4 \_\_manOf 多选](#54-__manof-多选)
    - [5.5 \_\_snowflakeId 雪花ID](#55-__snowflakeid-雪花id)
  - [6. SO 插件](#6-so-插件)
    - [6.1 通用调用语法](#61-通用调用语法)
    - [6.2 login 插件](#62-login-插件)
    - [6.3 aes 插件](#63-aes-插件)
  - [7. 表达式与变量引用](#7-表达式与变量引用)
    - [7.1 变量引用](#71-变量引用)
    - [7.2 函数调用](#72-函数调用)
    - [7.3 数学表达式](#73-数学表达式)
    - [7.4 混合使用](#74-混合使用)
    - [7.5 引号规则](#75-引号规则)
  - [8. 节点通用字段](#8-节点通用字段)
    - [8.1 block\_on\_error 错误阻断](#81-block_on_error-错误阻断)
  - [9. 变量默认值与 Payload 类型匹配](#9-变量默认值与-payload-类型匹配)
    - [9.1 字符串字段（后端需要 `"value"`）](#91-字符串字段后端需要-value)
    - [9.2 整数字段（后端需要 `123`）](#92-整数字段后端需要-123)
    - [9.3 常见错误与排查](#93-常见错误与排查)
    - [9.4 速查表](#94-速查表)
  - [10. 完整示例](#10-完整示例)
  - [附：节点类型速查表](#附节点类型速查表)

---

## 1. YAML 顶层结构

```yaml
name: 场景名称                    # 必填，场景标识
description: |                  # 可选，场景描述
  多行描述文本
variables:                      # 可选，场景变量(运行时常量)
  - key: var_name
    value: "var_value"
config_params:                  # 可选，配置参数(与 variables 合并存储)
  param1: "value1"
derived_params:                 # 可选，派生参数(与 variables 合并存储)
  param2: "value2"
data_sources:                   # 可选，CSV 数据源
  - name: source_name
    columns: [col1, col2]
    rows:
      - col1: "v1"
        col2: "v2"
setup:                          # 可选，Setup 阶段节点(顺序执行)
  - name: ...
    type: ...
    config: {}
nodes:                          # 必填，主流程节点
  - name: ...
    type: ...
    config: {}
teardown:                       # 可选，Teardown 阶段节点(顺序执行)
  - name: ...
    type: ...
    config: {}
edges:                          # 可选，DAG 边定义(省略时按 setup→nodes→teardown 顺序连接)
  - from: 节点A
    to: 节点B
    condition: "__if_true__"    # 可选，边条件
```

> **关键说明**：`variables` / `config_params` / `derived_params` 三者在导入时合并为同一个 `Variables` JSON 对象存储，无功能差异，仅用于组织来源不同的变量。

参考实现：[handler.go#L129-L144]($PROJECT_HOME/salvo/internal/api/handler.go#L129-L144)

---

## 2. DAG 执行模型

Salvo 的节点编排基于 **有向无环图（DAG）** 模型。每个节点是图中的一个顶点，`edges` 定义了顶点之间的有向边。执行引擎根据 DAG 拓扑排序决定执行顺序。

### 2.1 拓扑排序与顺序执行

当未定义 `edges` 时，Salvo 自动按 `setup → nodes → teardown` 顺序串联执行，等价于一条线性链。

当定义了 `edges` 时，执行引擎对 DAG 进行**拓扑排序**，按依赖关系依次执行节点。只有当前驱节点全部执行完毕后，后续节点才会被触发。

```yaml
# 线性链：A → B → C → D
edges:
  - from: A
    to: B
  - from: B
    to: C
  - from: C
    to: D
```

### 2.2 独立分支并行

当 DAG 中存在**没有依赖关系的独立分支**时，执行引擎会**自动并行**执行这些分支，无需手动配置。

```yaml
# A → B → D
# A → C → D
# B 和 C 无依赖关系，会并行执行
edges:
  - from: A
    to: B
  - from: A
    to: C
  - from: B
    to: D
  - from: C
    to: D
```

执行顺序：
1. 执行 A
2. A 完成后，B 和 C **并行**执行
3. B 和 C 都完成后，执行 D

> **注意**：如果 B 和 C 之间有数据依赖（如 B 提取的变量被 C 使用），则不能并行，必须通过边显式建立依赖关系。

### 2.3 边条件控制

`edges` 的 `condition` 字段可以控制分支走向，配合 `if-else` 节点实现条件分支：

| condition 值 | 说明 |
|-------------|------|
| `__if_true__` | 当 if-else 节点的 `expr` 求值为 true 时走此边 |
| `__if_false__` | 当 if-else 节点的 `expr` 求值为 false 时走此边 |
| 省略 | 无条件执行（默认） |

```yaml
edges:
  - from: 判断支付状态
    to: 执行支付
    condition: "__if_true__"
  - from: 判断支付状态
    to: 跳过支付
    condition: "__if_false__"
```

参考实现：
- DAG 拓扑排序：[dag.go]($PROJECT_HOME/salvo/internal/core/dag/dag.go)
- 执行引擎：[executor.go]($PROJECT_HOME/salvo/internal/runner/runner.go)

---

## 3. 场景变量与数据源

### 3.1 variables 场景变量

场景变量在场景启动时初始化，所有节点共享。引用方式：`${var_name}`。

```yaml
variables:
  # 基础配置
  - key: base_url
    value: "https://uat.example.com"
  - key: timeout_ms
    value: "10000"
  # 运行时变量(初始为空，节点执行中填充)
  - key: token
    value: ""
  - key: order_id
    value: ""
```

变量支持**嵌套引用**（最多 10 层）：

```yaml
variables:
  - key: env
    value: "uat"
  - key: api_host
    value: "${env}-api.example.com"      # → "uat-api.example.com"
  - key: base_url
    value: "http://${api_host}/api"       # → "http://uat-api.example.com/api"
```

### 3.2 config_params 配置参数

`config_params` 是 map 形式的变量，导入时与 `variables` 合并存储，主要用于**业务配置参数**。

```yaml
config_params:
  charge_time: "300"
  pay_type: "1"
  pay_channel: "2"
  env_type: "uat1"
```

在节点中引用：`${charge_time}` / `${pay_type}` / `${env_type}`

### 3.3 derived_params 派生参数

`derived_params` 同样合并到 `variables`，主要用于**派生/计算参数**。当前导入时不会执行表达式，仅作为普通字符串存储。

```yaml
derived_params:
  max_charge_power_kw: "30"
  charge_post_offline_time: "150"
```

### 3.4 data_sources CSV 数据源

数据源用于参数化测试，每个虚拟用户从数据源中取一行数据，通过 `${数据源名.列名}` 引用。

```yaml
data_sources:
  - name: users                          # 数据源名称
    columns:                             # 列定义
      - phone
      - cardnum
      - b2b_cardpwd
    rows:                                # 数据行(可多行)
      - phone: "15550000000"
        cardnum: "15550000000"
        b2b_cardpwd: "Abcd!@#$1234"
      - phone: "15550000001"
        cardnum: "15550000001"
        b2b_cardpwd: "Abcd!@#$1234"

  - name: chargers
    columns:
      - charger
      - pointId
      - equipNo
    rows:
      - charger: "1100000001"
        pointId: "1100000001"
        equipNo: "1100000001"
```

引用示例：

```yaml
body: '{"phone":"${users.phone}","cardNum":"${users.cardnum}"}'
body: '{"charger":"${chargers.charger}","pointId":"${chargers.pointId}"}'
```

#### 2.4.1 YAML 与 CSV 数据源共存

同一场景中，同名数据源可以同时存在 YAML 版本（`source=yaml`）和 CSV 版本（`source=csv`），两者独立存储、互不覆盖。

**共存规则**：

| 操作 | 行为 |
|------|------|
| **YAML 导入** | 若同名 CSV 已存在 → 跳过（保留 CSV）；若同名 YAML 已存在 → 覆盖旧 YAML |
| **CSV 上传** | 若同名 CSV 已存在 → 覆盖旧 CSV；若同名 YAML 已存在 → 保留 YAML（不覆盖） |
| **节点执行时取值** | 同名数据源中 **CSV 优先**，CSV 不存在时 fallback 到 YAML |

**典型场景**：

YAML 中定义了 10 条测试数据，又通过 GUI 上传了同名的 CSV 文件（包含 3 条特定数据）：

- GUI 数据源列表中会显示两条记录：`users` (yaml配置) + `users` (csv上传)
- 节点执行时引用 `${users.phone}` 优先使用 CSV 的 3 条数据
- CSV 数据只有 3 行，但 LOOP 30 次：RowIterator 会**循环轮询**，第 4 次取第 1 行，第 5 次取第 2 行，依此类推
- 删除 CSV 数据源后，自动 fallback 到 YAML 的 10 条数据

**RowIterator 循环取值**：

数据源行迭代器在所有行遍历完毕后自动回到第一行（round-robin），因此数据行数不必与循环次数匹配：

```
数据源有 3 行: [row1, row2, row3]
循环 7 次的取值顺序: row1 → row2 → row3 → row1 → row2 → row3 → row1
```

参考实现：[handler.go#L162-L181]($PROJECT_HOME/salvo/internal/api/handler.go#L162-L181)

---

## 4. 节点类型详解

所有节点共享通用字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 节点名称(唯一标识) |
| `type` | string | 节点类型 |
| `config` | object | 节点配置(类型相关) |
| `think_time` | string | 思考时间，如 `"1000ms"` |
| `retry` | object | 重试配置 `{count, interval}` |
| `condition` | string | 节点执行条件表达式 |
| `timed_trigger` | string | 定时触发，如 `"@every 10s"` |
| `loop_count` | int | 节点循环次数(顶层字段) |
| `block_on_error` | bool | 节点失败时是否中断整个链路执行（默认 `false`） |

节点类型清单（共 13 种）：

| 类型 | 说明 | 典型用途 |
|------|------|----------|
| `http` | HTTP 请求 | 业务接口调用 |
| `setup` | Setup 阶段 HTTP | 初始化(登录、注册) |
| `teardown` | Teardown 阶段 HTTP | 清理(删除数据) |
| `generator` | 生成器 | 调用函数/SO 插件生成变量 |
| `delay` | 延迟 | 固定等待 |
| `timer` | 定时器 | 延迟/间隔触发 |
| `condition` | 条件判断 | 单纯条件求值 |
| `if-else` | 条件分支 | 二选一路径 |
| `group` | 分组 | 子节点循环 |
| `while` | 循环(条件退出) | 轮询充电状态 |
| `loop` | 循环(固定次数) | 批量执行 |
| `parallel` | 并行 | 并发请求 |
| `sub_flow` | 子流程 | 嵌套场景调用 |

### 4.1 http HTTP 请求节点

最常用的节点类型，执行一次 HTTP 请求。

```yaml
- name: 创建订单
  type: http
  config:
    method: POST
    url: "${base_url}/orders"
    headers:
      Content-Type: application/json
      Authorization: "Bearer ${token}"
    body: '{"user_id":"${user_id}","amount":99.99}'
    timeout: 10000                # 秒(数字) 或 毫秒字符串"${timeout_ms}"
    expect_body:                  # 可选，响应体断言
      errorCode: 0
    extract:                      # 可选，从响应提取变量
      order_id: "$.result.orderId"
      token: "$.data.token"
  think_time: "1000ms"            # 可选，思考时间
  retry:                          # 可选，重试配置
    count: 3
    interval: "500ms"
  condition: "${order_id} != ''"  # 可选，执行条件
```

**字段说明**：

| 字段 | 必填 | 说明 |
|------|------|------|
| `method` | 否 | HTTP 方法，默认 `GET` |
| `url` | 是 | 请求 URL，支持 `${var}` |
| `headers` | 否 | 请求头 map |
| `body` | 否 | 请求体字符串(JSON 字符串) |
| `form` | 否 | multipart/form-data 表单字段，支持文件上传（见下方示例） |
| `timeout` | 否 | 超时(秒为数字，毫秒为字符串) |
| `expect_body` | 否 | 响应体 JSON 字段断言 |
| `extract` | 否 | JSONPath 提取变量 |
| `aes_decrypt` | 否 | AES 密钥（base64），用于解密加密的响应体 |
| `aes_mode` | 否 | AES 模式：`0`=CBC，`1`=GCM（默认） |

**form-data 文件上传**：

```yaml
- name: 上传文件
  type: http
  config:
    method: POST
    url: "${base_url}/upload"
    form:
      file:
        file_path: "/path/to/test.pdf"
        content_type: "application/pdf"
      description: "测试文件上传"
```

> **注意**：`form` 和 `body` 互斥，使用 `form` 时自动设置 `Content-Type: multipart/form-data`。`file_path` 指向本地文件，`content_type` 可选。

**aes_decrypt 响应解密**：

```yaml
# 步骤响应体是 AES-GCM 加密的，需要先解密再提取变量
- name: 查询加密数据
  type: http
  config:
    method: GET
    url: "${base_url}/encrypted-data"
    aes_decrypt: "${aes_key_base64}"     # base64 编码的 AES 密钥
    aes_mode: 1                          # GCM 模式（默认）
    extract:
      secret_value: "$.data.value"
```

**extract 两种写法**：

```yaml
# 写法1: map 形式(推荐，简洁)
extract:
  order_id: "$.result.orderId"
  token: "$.data.token"

# 写法2: 列表形式(while/loop/parallel 内部 steps 使用)
extract:
  - variable: order_id
    path: "$.result.orderId"
  - variable: token
    path: "$.data.token"
```

**retry 重试配置**：

```yaml
retry:
  count: 3           # 最大重试次数
  interval: "500ms"  # 重试间隔
```

> **注意**：`body` 必须是字符串，内部 JSON 用单引号包裹。`extract` 仅在 http/setup/teardown 节点的 `config` 中有效；while/loop/parallel 内部 steps 的 extract 在 step 层级。

参考实现：[runner.go#L1371-L1612]($PROJECT_HOME/salvo/internal/runner/runner.go#L1371-L1612)

---

### 4.2 setup / teardown 生命周期节点

`setup` 和 `teardown` 在导入时被识别为生命周期节点，分别放入 Setup 阶段和 Teardown 阶段。它们本质上也是 HTTP 节点，配置完全相同。

```yaml
setup:
  - name: 登录获取Token
    type: setup
    config:
      method: POST
      url: "${base_url}/auth/login"
      headers:
        Content-Type: application/json
      body: '{"username":"${username}","password":"${password}"}'
      timeout: 10
      extract:
        token: "$.data.token"
        user_id: "$.data.user.id"

teardown:
  - name: 清理测试数据
    type: teardown
    config:
      method: DELETE
      url: "${base_url}/users/${user_id}"
      headers:
        Authorization: "Bearer ${token}"
      timeout: 5
```

**执行顺序**：

- 有 `edges`：按 edges 定义的 DAG 拓扑执行
- 无 `edges`：Setup → Nodes → Teardown 顺序串联执行

参考实现：[handler.go#L543-L549]($PROJECT_HOME/salvo/internal/api/handler.go#L543-L549)

---

### 4.3 generator 生成器节点

执行表达式并将结果存入变量。主要用于：
1. 调用内置函数（`__random`、`__snowflakeId` 等）
2. 调用 SO 插件（`__so`）
3. 执行数学运算

```yaml
- name: 生成订单号
  type: generator
  config:
    expression: '${__snowflakeId()}'
    variable: order_id        # 结果存入此变量

- name: 生成随机充电时长
  type: generator
  config:
    expression: '${__random(60, 600)}'
    variable: charge_time

- name: 概率性是否强制停止
  type: generator
  config:
    expression: '${__weightedChoice("1=35,0=65")}'
    variable: force_stop_charge

- name: 调用 login 插件
  type: generator
  config:
    expression: '${__so("login","login","${salt_url}","${login_url}","${username}","${password}")}'
    variable: jwt_token

- name: 数学运算
  type: generator
  config:
    expression: '${max_charge_power} / 1000'
    variable: max_charge_power_kw
```

**字段说明**：

| 字段 | 必填 | 说明 |
|------|------|------|
| `expression` | 是 | 表达式字符串，支持 `${func(args)}` / `${var}` / 数学运算 |
| `variable` | 是 | 结果写入的变量名 |

**执行流程**：
1. 解析 `${generator.xxx}` 引用
2. 解析 `${__funcName(args)}` 函数调用
3. 解析 `${varName}` 变量引用
4. 若结果为纯数学表达式，求值
5. 将最终结果写入 `variable`

参考实现：[runner.go#L1614-L1700]($PROJECT_HOME/salvo/internal/runner/runner.go#L1614-L1700)

---

### 4.4 delay 延迟节点

固定时间延迟，用于模拟用户操作间隔。

```yaml
- name: 用户思考延迟
  type: delay
  config:
    ms: 500                    # 毫秒
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `ms` | 是 | 延迟毫秒数 |

> 若 `ms <= 0`，默认 100ms。

参考实现：[runner.go#L1697-L1732]($PROJECT_HOME/salvo/internal/runner/runner.go#L1697-L1732)

---

### 4.5 timer 定时器节点

支持两种模式：延迟(delay)和间隔(interval)。

```yaml
# 模式1: 延迟(等同于  节点，但支持秒)
- name: 支付后等待
  type: timer
  config:
    mode: delay
    seconds: 1.5                # 秒

# 模式2: 间隔触发(周期性 tick)
- name: 心跳检测
  type: timer
  config:
    mode: interval
    interval: 2                 # 每 2 秒触发一次
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `mode` | 是 | `delay` 或 `interval` |
| `seconds` | 否 | 秒数(浮点) |
| `delay` | 否 | 同 `seconds`(别名) |
| `duration` | 否 | 同 `seconds`(别名) |
| `interval` | 否 | 同 `seconds`(别名) |

> 字段优先级：`seconds` > `delay` > `duration` > `interval`。若都为 0，默认 1 秒。

参考实现：[runner.go#L1860-L1960]($PROJECT_HOME/salvo/internal/runner/runner.go#L1860-L1960)

---

### 4.6 condition 条件判断节点

求值一个条件表达式，结果存入节点输出。**单纯求值，不影响分支**。若需分支，请用 `if-else`。

```yaml
- name: 检查订单是否存在
  type: condition
  config:
    expr: "${order_id} != ''"
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `expr` | 是 | 条件表达式，详见[条件表达式语法](#条件表达式语法) |

**条件表达式语法**：

| 格式 | 示例 | 说明 |
|------|------|------|
| `${var} == "value"` | `${status} == "4"` | 字符串相等 |
| `${var} != "value"` | `${status} != "COMPLETED"` | 字符串不等 |
| `${var} > N` | `${count} > 10` | 数值大于 |
| `${var} >= N` | `${count} >= 10` | 数值大于等于 |
| `${var} < N` | `${count} < 5` | 数值小于 |
| `${var} <= N` | `${count} <= 5` | 数值小于等于 |
| `${var}` | `${order_id}` | 真值检查(非空) |
| `!${var}` | `!${order_id}` | 空值检查 |

支持的运算符：`==` `!=` `>` `>=` `<` `<=` 以及别名 `equals` `not_equals` `greater_than` `less_than` `empty` `not_empty` 等。

参考实现：[runner.go#L1748-L1772]($PROJECT_HOME/salvo/internal/runner/runner.go#L1748-L1772)

---

### 4.7 if-else 条件分支节点

二选一分支。节点本身只求值 `expr`，**真正的分支由 edges 的 `condition` 字段控制**。

```yaml
nodes:
  - name: 判断是否支付
    type: if-else
    config:
      expr: "${order_id} != ''"

  - name: 支付订单
    type: http
    config:
      method: POST
      url: "${base_url}/payment"
      body: '{"order_id":"${order_id}"}'

  - name: 跳过支付
    type: http
    config:
      method: GET
      url: "${base_url}/orders/${order_id}"

edges:
  - from: 判断是否支付
    to: 支付订单
    condition: "__if_true__"     # expr 为 true 时走此边
  - from: 判断是否支付
    to: 跳过支付
    condition: "__if_false__"    # expr 为 false 时走此边
```

**关键约定**：
- if-else 节点必须有两条出边：`condition: "__if_true__"` 和 `condition: "__if_false__"`
- `expr` 语法同 [condition 节点](#36-condition-条件判断节点)

参考实现：[runner.go#L1774-L1798]($PROJECT_HOME/salvo/internal/runner/runner.go#L1774-L1798)

---

### 4.8 group 分组节点

将多个子节点组合成一个组，支持循环和异步执行。

```yaml
nodes:
  - name: 校验库存
    type: http
    config: { method: GET, url: "${base_url}/inventory/${product_id}" }

  - name: 创建订单
    type: http
    config: { method: POST, url: "${base_url}/orders", body: '{}' }

  - name: 订单处理流程
    type: group
    config:
      node_ids:                  # 子节点名称列表(按顺序执行)
        - 校验库存
        - 创建订单
      loop_count: 3              # 循环 3 次
      async: false              # 是否异步(默认 false)
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `node_ids` | 是 | 子节点名称列表 |
| `loop_count` | 否 | 循环次数，默认 1 |
| `async` | 否 | 是否异步执行，默认 false |

**说明**：
- 子节点在 `nodes` 中正常定义，group 通过 `node_ids` 引用
- `async: true` 时子节点在后台执行，group 立即返回
- 导入时会自动将 `node_ids` 从名称解析为节点 ID

参考实现：[runner.go#L1783-L1859]($PROJECT_HOME/salvo/internal/runner/runner.go#L1783-L1859)

---

### 4.9 while 循环节点

条件退出循环，常用于轮询场景。

```yaml
- name: 轮询充电状态
  type: while
  config:
    exit_conditions:             # 退出条件(任一满足即退出)
      - variable: charging_status
        operator: equals
        value: "4"
    interval_seconds: 30         # 每轮间隔(秒)
    max_iterations: 100         # 最大迭代次数
    max_duration_minutes: 60     # 最大持续时间(分钟)
    fail_after_consecutive: 10   # 连续失败 N 次则失败
    fail_message: "启动超时"
    steps:                       # 循环体内步骤
      - name: 查询充电状态
        request:
          method: POST
          url: "${base_url}/charge/status"
          headers:
            Authorization: "${token}"
          body: '{"seq":"${start_charge_seq}"}'
        extract:
          - variable: charging_status
            path: "$.result.status"
          - variable: charge_kwh
            path: "$.result.chargeKwh"
        think_time:               # 思考时间(毫秒)
          min: 1000
          max: 3000

      - name: 桩离线(条件步骤)
        condition:
          variable: charge_post_offline
          operator: equals
          value: "1"
        request:
          method: POST
          url: "${sim_url}/station/status"
          body: '{"status":2}'

      - name: 桩上线(定时触发)
        condition:
          variable: charge_post_offline
          operator: equals
          value: "1"
        timed_trigger:
          after_seconds: "${charge_post_online_time}"
          once: true
        request:
          method: POST
          url: "${sim_url}/station/status"
          body: '{"status":0}'

      - name: 停止充电(条件+重试)
        condition:
          variable: charge_time_reached
          operator: equals
          value: "true"
        request:
          method: POST
          url: "${base_url}/stop"
          body: '{"seq":"${start_charge_seq}"}'
        retry:
          max_attempts: 10
          on_429: retry
```

**字段说明**：

| 字段 | 必填 | 说明 |
|------|------|------|
| `exit_conditions` | 是 | 退出条件列表 |
| `interval_seconds` | 否 | 每轮间隔秒数 |
| `max_iterations` | 否 | 最大迭代次数 |
| `max_duration_minutes` | 否 | 最大持续分钟数 |
| `fail_on_max_iterations` | 否 | 达到最大迭代次数时是否视为失败，默认 `true`。设为 `false` 则视为成功退出（如轮询占位订单场景） |
| `fail_on_max_duration` | 否 | 达到最大持续时间时是否视为失败，默认 `true`。设为 `false` 则视为成功退出 |
| `fail_after_consecutive` | 否 | 连续失败次数阈值 |
| `fail_message` | 否 | 失败消息 |
| `steps` | 是 | 循环体步骤列表 |

**step 字段**：

| 字段 | 说明 |
|------|------|
| `name` | 步骤名称 |
| `type` | 步骤类型：`"http"`（默认）或 `"generator"`（生成器步骤，使用 `config.expression` + `config.variable`） |
| `condition` | 步骤执行条件 `{variable, operator, value}` |
| `request` | HTTP 请求 `{method, url, headers, body, form, expect_body}` |
| `extract` | 变量提取(列表形式) |
| `think_time` | 思考时间 `{min, max}`(毫秒) |
| `timed_trigger` | 定时触发 `{after_seconds, once}` |
| `retry` | 重试 `{max_attempts, on_429}` |
| `block_on_error` | 步骤失败时是否立即中断 while 循环（详见 [8.1 block_on_error](#81-block_on_error-错误阻断)） |
| `aes_decrypt` | AES 密钥（base64），用于解密加密的响应体 |
| `aes_mode` | AES 模式：`0`=CBC，`1`=GCM（默认）。nil 时默认 GCM |

**operator 支持的值**：`equals` `not_equals` `greater_than` `greater_than_or_equal` `less_than` `less_than_or_equal` `empty` `not_empty`

**fail_on_max_iterations 示例**：

```yaml
# 轮询占位订单场景：达到最大迭代视为成功（订单已占位即目标达成）
- name: 轮询占位订单
  type: while
  config:
    exit_conditions:
      - variable: order_status
        operator: equals
        value: "PLACED"
    max_iterations: 30
    fail_on_max_iterations: false     # 达到 30 次视为成功，记录 WARN 日志
    fail_message: "占位超时但可接受"
    steps:
      - name: 查询订单状态
        request:
          method: GET
          url: "${base_url}/orders/${order_id}/status"
        extract:
          - variable: order_status
            path: "$.data.status"
```

**aes_decrypt 响应解密示例**：

```yaml
# 步骤响应体是 AES-GCM 加密的，需要先解密再提取变量
- name: 查询加密数据
  aes_decrypt: "${aes_key_base64}"     # base64 编码的 AES 密钥
  aes_mode: 1                          # GCM 模式（默认）
  request:
    method: GET
    url: "${base_url}/encrypted-data"
  extract:
    - variable: secret_value
      path: "$.data.value"
```

参考实现：[while_node.go#L1-L130]($PROJECT_HOME/salvo/internal/runner/while_node.go#L1-L130)

---

### 4.10 loop 循环节点

固定次数循环，与 while 的区别是**无退出条件**，固定循环 N 次。

```yaml
- name: 批量创建订单
  type: loop
  config:
    loop_count: 5               # 循环 5 次
    steps:
      - name: 生成订单ID
        request:
          method: GET
          url: "${base_url}/gen-id"
        extract:
          - variable: order_id
            path: "$.data.id"
      - name: 创建订单
        request:
          method: POST
          url: "${base_url}/orders"
          body: '{"id":"${order_id}"}'
        think_time:
          min: 500
          max: 1500
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `loop_count` | 是 | 循环次数 |
| `steps` | 是 | 步骤列表(语法同 while) |

参考实现：[loop_node.go#L1-L100]($PROJECT_HOME/salvo/internal/runner/loop_node.go#L1-L100)

---

### 4.11 parallel 并行节点

多个步骤并发执行，提取的变量合并回主作用域。

```yaml
- name: 并行查询用户和订单
  type: parallel
  config:
    steps:
      - name: 查询用户信息
        request:
          method: GET
          url: "${base_url}/users/${user_id}"
        extract:
          - variable: user_name
            path: "$.data.name"
      - name: 查询订单列表
        request:
          method: GET
          url: "${base_url}/orders?user=${user_id}"
        extract:
          - variable: order_count
            path: "$.data.total"
      - name: 条件步骤(可选)
        condition:
          variable: need_detail
          operator: equals
          value: "1"
        request:
          method: GET
          url: "${base_url}/users/${user_id}/detail"
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `steps` | 是 | 并行步骤列表 |

**特点**：
- 所有步骤共享初始变量(隔离写入)
- 步骤执行完毕后，提取的变量合并回主作用域
- 支持每个步骤的 `condition`(不满足则跳过该步骤)

参考实现：[parallel_node.go#L1-L100]($PROJECT_HOME/salvo/internal/runner/parallel_node.go#L1-L100)

---

### 4.12 sub_flow 子流程节点

调用另一个场景作为子流程，支持同步/异步。

```yaml
- name: 调用登录子流程
  type: sub_flow
  config:
    scene_id: "1234567890"       # 目标场景 ID
    async: false                 # 是否异步(默认 false)
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `scene_id` | 是 | 子场景 ID |
| `async` | 否 | 异步模式(后台执行)，默认 false |

**限制**：
- 最大嵌套深度 5 层
- 自动检测循环引用(同一场景不能在链路中重复出现)

参考实现：[sub_flow_node.go#L1-L100]($PROJECT_HOME/salvo/internal/runner/sub_flow_node.go#L1-L100)

---

## 5. 内置系统函数

内置函数通过 `${__funcName(args)}` 语法调用，可在以下位置使用：
- `generator` 节点的 `expression`
- `http` 节点的 `url` / `body` / `headers` 值
- 其他节点的配置字段

注册位置：[register.go]($PROJECT_HOME/salvo/internal/generator/builtin/register.go)

### 5.1 \_\_random 随机数

生成随机数，支持整数和浮点数两种模式。

**语法**：`${__random(min, max)}` 或 `${__random(min, max, scale)}`

```yaml
# 示例1: 整数随机(60~600)
- name: 随机充电时长
  type: generator
  config:
    expression: '${__random(60, 600)}'
    variable: charge_time

# 示例2: 浮点数随机(0.00~100.00，保留2位小数)
- name: 随机金额
  type: generator
  config:
    expression: '${__random(0, 100, 2)}'
    variable: amount

# 示例3: 在 URL 中直接使用
- name: 随机分页
  type: http
  config:
    method: GET
    url: "${base_url}/products?page=${__random(1, 10)}"
```

**参数说明**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `min` | number | 最小值 |
| `max` | number | 最大值 |
| `scale` | int | 可选，小数位数。省略则为整数模式 |

> 整数模式：返回 `[ceil(min), floor(max)]` 范围内的整数。
> 浮点模式：返回 `min + rand * (max-min)`，格式化为 `scale` 位小数。

参考实现：[random.go]($PROJECT_HOME/salvo/internal/generator/builtin/random.go)

---

### 5.2 \_\_weightedChoice 加权选择

按权重随机选择一个选项。

**语法**：`${__weightedChoice("opt1=w1,opt2=w2,...")}`

```yaml
# 示例1: 35% 概率为 1，65% 概率为 0
- name: 是否强制停止
  type: generator
  config:
    expression: '${__weightedChoice("1=35,0=65")}'
    variable: force_stop_charge

# 示例2: 三选一(权重不必加和为100，自动归一化)
- name: 选择支付方式
  type: generator
  config:
    expression: '${__weightedChoice("wechat=40,alipay=35,cash=25")}'
    variable: pay_method

# 示例3: 在 body 中直接使用
- name: 创建订单
  type: http
  config:
    method: POST
    url: "${base_url}/orders"
    body: '{"channel":"${__weightedChoice(\"card=70,cash=30\")}"}'
```

**参数格式**：`"key1=weight1,key2=weight2,..."`，权重为正数，自动归一化。

参考实现：[weighted_choice.go]($PROJECT_HOME/salvo/internal/generator/builtin/weighted_choice.go)

---

### 5.3 \_\_oneOf 等概率选择

从多个选项中随机选一个，每个选项等概率。

**语法**：`${__oneOf("opt1","opt2","opt3",...)}`

> **类型说明**：`__oneOf` 入参和返回值均为**字符串**。即使传整数枚举（如 `1,2,3`），也会被当作字符串 `"1"` / `"2"` / `"3"` 处理。最终在 JSON body 中是字符串还是数字，取决于是否被引号包裹：
> - 字符串：`'{"errorCode":"${__oneOf(\"1\",\"2\")}"}'` → `{"errorCode":"2"}`
> - 数字：`'{"errorCode":${__oneOf(\"1\",\"2\")}}'` → `{"errorCode":2}`

```yaml
# 示例1: 随机选择错误码(整数枚举也用引号包裹，返回字符串)
- name: 随机错误码
  type: generator
  config:
    expression: '${__oneOf("2","3","4","5")}'
    variable: error_code

# 示例2: 随机选择城市
- name: 随机城市
  type: generator
  config:
    expression: '${__oneOf("beijing","shanghai","guangzhou","shenzhen")}'
    variable: city

# 示例3: 在 URL 中使用
- name: 随机查询
  type: http
  config:
    method: GET
    url: "${base_url}/search?city=${__oneOf(\"bj\",\"sh\",\"gz\")}"
```

参考实现：[one_of.go]($PROJECT_HOME/salvo/internal/generator/builtin/one_of.go)

---

### 5.4 \_\_manOf 多选

从多个选项中**独立**以 50% 概率选择每个选项，返回逗号分隔的字符串。保证至少选一个。

**语法**：`${__manOf("opt1","opt2","opt3",...)}`

```yaml
# 示例1: 随机选择评论标签(1~7 中随机多个)
- name: 随机评论标签
  type: generator
  config:
    expression: '${__manOf("1","2","3","4","5","6","7")}'
    variable: comment_tags

# 示例2: 随机选择兴趣爱好
- name: 随机兴趣
  type: generator
  config:
    expression: '${__manOf("reading","music","sports","travel","food")}'
    variable: hobbies
```

**返回值**：逗号分隔的字符串，如 `"1,3,5"` 或 `"reading,music"`。保证至少返回一个选项。

参考实现：[man_of.go]($PROJECT_HOME/salvo/internal/generator/builtin/man_of.go)

---

### 5.5 \_\_snowflakeId 雪花ID

生成全局唯一的雪花 ID(19 位数字字符串)。

**语法**：`${__snowflakeId()}`

```yaml
# 示例1: 生成订单号
- name: 生成订单号
  type: generator
  config:
    expression: '${__snowflakeId()}'
    variable: order_id

# 示例2: 生成任务ID
- name: 生成任务ID
  type: generator
  config:
    expression: '${__snowflakeId()}'
    variable: task_id

# 示例3: 在 body 中使用
- name: 创建任务
  type: http
  config:
    method: POST
    url: "${base_url}/tasks"
    body: '{"taskId":"${__snowflakeId()}","name":"test"}'
```

参考实现：[snowflake.go]($PROJECT_HOME/salvo/internal/generator/builtin/snowflake.go)

---

## 6. SO 插件

SO 插件是编译为 `.so` 共享库的 Go 插件，通过 `__so` 函数调用。

### 6.1 通用调用语法

```
${__so("pluginName", "operation", arg1, arg2, ...)}
${__so("pluginName@version", "operation", arg1, ...)}   # 指定版本
```

**说明**：
- 第 1 个参数：插件名称(可选 `@version` 后缀)
- 第 2 个参数：操作名称
- 后续参数：传递给操作的参数(字符串)

插件加载位置参考：[loader.go]($PROJECT_HOME/salvo/internal/plugin/so/loader.go)

---

### 6.2 login 插件

B 端登录插件，完整流程：AES-GCM 加密 → 获取 salt → bcrypt 哈希 → 登录 → 返回 JWT。

**位置**：[plugins/login/main.go]($PROJECT_HOME/salvo/plugins/login/main.go)

**操作清单**：

| 操作 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `login` | `salt_base_url, login_base_url, username, password` | JWT token | 完整登录流程 |
| `encrypt_username` | `key, iv_base64, plaintext` | base64 密文 | AES-GCM 加密 |
| `get_salt` | `base_url, encrypted_username` | JSON 字符串 | 获取 salt |
| `decrypt_salt` | `secret_key, iv, salt_str` | 明文 salt | AES-GCM 解密 |
| `bcrypt_hash` | `password, salt` | bcrypt 哈希 | bcrypt 哈希 |
| `build_login_info` | `secret_key, iv, hashed_password` | base64 密文 | 构造登录信息 |
| `username_login` | `login_url, login_info, enc_username, secret_key` | JWT token | 用户名登录 |

**示例**：

```yaml
# 示例1: 完整登录(推荐)
variables:
  - key: salt_base_url
    value: "https://uat.example.com/jv/tob-adapter/tob-adapter-application/crm/jv/auth/v1"
  - key: login_base_url
    value: "https://uat.example.com/jv/tob-adapter/jv/auth/v1"
  - key: username
    value: "18936879143"
  - key: password
    value: "Abcd!@#$1234"

setup:
  - name: 登录获取JWT
    type: generator
    config:
      expression: '${__so("login","login","${salt_base_url}","${login_base_url}","${username}","${password}")}'
      variable: jwt_token

# 示例2: 单独加密用户名(高级用法)
- name: 加密用户名
  type: generator
  config:
    expression: '${__so("login","encrypt_username","mykey","iv_base64=","myuser")}'
    variable: enc_username

# 示例3: 单独 bcrypt 哈希
- name: 密码哈希
  type: generator
  config:
    expression: '${__so("login","bcrypt_hash","mypassword","$2a$10$salt")}'
    variable: hashed_pwd
```

---

### 6.3 aes 插件

AES-CBC 加密/解密插件。

**位置**：[plugins/aes/main.go]($PROJECT_HOME/salvo/plugins/aes/main.go)

**操作清单**：

| 操作 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `encrypt` | `key, iv_base64, plaintext` | base64 密文 | AES-CBC 加密(PKCS7 填充) |
| `decrypt` | `key, iv_base64, ciphertext_base64` | 明文 | AES-CBC 解密 |

**示例**：

```yaml
# 示例1: 加密
- name: AES加密
  type: generator
  config:
    expression: '${__so("aes","encrypt","mykey32bytesecretkey123456","iv_base64=","hello")}'
    variable: encrypted

# 示例2: 解密
- name: AES解密
  type: generator
  config:
    expression: '${__so("aes","decrypt","mykey32bytesecretkey123456","iv_base64=","base64ciphertext==")}'
    variable: decrypted

# 示例3: 加密后用于请求体
- name: 加密敏感数据
  type: generator
  config:
    expression: '${__so("aes","encrypt","${aes_key}","${aes_iv}","${plaintext_data}")}'
    variable: enc_data
- name: 发送加密数据
  type: http
  config:
    method: POST
    url: "${base_url}/secure"
    body: '{"data":"${enc_data}"}'
```

---

## 7. 表达式与变量引用

Salvo 表达式引擎支持三种 `${...}` 表达式：

### 7.1 变量引用

```
${variable_name}
${data_source.column}              # CSV 数据源列
${nested_var}                       # 嵌套变量(变量值中含 ${})
```

### 7.2 函数调用

```
${__functionName(args)}
${__functionName("str_arg", 123, ${var})}
```

### 7.3 数学表达式

```
${max_charge_power} / 1000
${price} * ${quantity}
(${base} + ${tax}) * 1.1
```

### 7.4 混合使用

```
${base_url}/orders/${__snowflakeId()}
Bearer ${__so("login","login",${salt_url},${login_url},${user},${pwd})}
```

### 7.5 引号规则

| 场景 | 写法 | 说明 |
|------|------|------|
| YAML 字符串值 | `expression: '${__random(1,10)}'` | 用单引号包裹 |
| 函数字符串参数 | `${__oneOf("a","b","c")}` | 双引号 |
| 包含变量的参数 | `${__so("p","op","${url}")}` | 嵌套 `${}` |
| body 中的 JSON | `body: '{"id":"${order_id}"}'` | 外层单引号，内层双引号 |

参考实现：[engine.go]($PROJECT_HOME/salvo/internal/core/expr/engine.go)

---

## 8. 节点通用字段

以下字段可应用于所有节点类型（在 `config` 之外）：

```yaml
- name: 节点名称
  type: http
  config: {}
  think_time: "1000ms"              # 思考时间
  retry:                             # 重试
    count: 3
    interval: "500ms"
  condition: "${var} > 0"           # 执行条件
  timed_trigger: "@every 10s"       # 定时触发
  loop_count: 3                     # 循环次数
```

**think_time 格式**：
- `"1000ms"` - 毫秒
- `"1s"` - 秒
- `{min: 1000, max: 3000}` - 随机范围(仅 while/loop/parallel 内部 steps)

参考实现：[handler.go#L485-L525]($PROJECT_HOME/salvo/internal/api/handler.go#L485-L525)

### 8.1 block_on_error 错误阻断

默认情况下，节点执行失败（HTTP 非 2xx、`expect_body` 断言失败等）不会中断整个链路，后续节点继续执行。通过设置 `block_on_error: true`，可以让该节点失败时**立即取消整个 chain 的执行**。

```yaml
# 示例：启动充电是关键步骤，失败后无需继续后续流程
- name: 启动充电
  type: http
  block_on_error: true              # 失败时中断整个链路
  config:
    method: POST
    url: "${baseurl}${api_start_charge}"
    headers:
      Content-Type: application/json
      Authorization: "${token}"
    body: '{"orderId":${order_id}}'
    expect_body:                    # 业务断言：errorCode 必须为 0
      errorCode: 0

# 示例：普通查询节点，失败不阻断（默认行为）
- name: 查询订单状态
  type: http
  # block_on_error 默认为 false，无需显式设置
  config:
    method: POST
    url: "${baseurl}${api_query_order}"
    body: '{"orderId":${order_id}}'
```

**触发阻断的两种场景**：

| 场景 | 触发条件 | 说明 |
|------|----------|------|
| HTTP 错误 | 响应状态码非 2xx（如 404、500） | 网络层/网关层错误 |
| 业务断言失败 | `expect_body` 中定义的字段值不匹配 | 业务层错误，如 `errorCode != 0` |

**与 while 循环的交互**：

while 循环内部的 steps 也支持 `block_on_error`，且**优先级高于 `fail_after_consecutive`**：

```yaml
- name: 轮询充电状态
  type: while
  config:
    exit_conditions:
      - variable: charging_status
        operator: equals
        value: "4"
    interval_seconds: 30
    max_iterations: 100
    fail_after_consecutive: 10      # 连续失败 10 次才退出
    steps:
      - name: 查询充电状态
        block_on_error: true        # 此步骤失败立即中断 while 循环
        request:
          method: POST
          url: "${baseurl}${api_query_status}"
          body: '{"orderId":${order_id}}'
        extract:
          - variable: charging_status
            path: "$.data.status"
```

**执行链路**：
1. step 失败 → 检查 `block_on_error: true` → 立即返回错误，中断 while 循环
2. while 节点返回错误 → DAG Executor 检查 while 节点自身的 `block_on_error`
3. 如果 while 节点也有 `block_on_error: true` → 取消整个 chain

**日志特征**：

节点执行日志会记录 `block_on_error` 状态，方便排查：

```json
{"msg":"node execution started","node_name":"启动充电","block_on_error":true}
{"msg":"chain cancelled due to block_on_error","node_id":"xxx","error":"HTTP 500: ..."}
{"msg":"node execution failed","node_name":"启动充电","block_on_error":true,"error":"..."}
```

参考实现：
- DAG 接口：[dag.go#L79-L82]($PROJECT_HOME/salvo/internal/core/dag/dag.go#L79-L82)
- Executor 链取消：[executor.go#L239-L250]($PROJECT_HOME/salvo/internal/runner/runner.go#L239-L250)
- HTTP 错误阻断：[runner.go#L1596-L1612]($PROJECT_HOME/salvo/internal/runner/runner.go#L1596-L1612)
- while 步骤阻断：[while_node.go#L314-L321]($PROJECT_HOME/salvo/internal/runner/while_node.go#L314-L321)

---

## 9. 变量默认值与 Payload 类型匹配

变量替换机制（`resolveWithVariables`）使用 `fmt.Sprintf("%v", v)` 将变量值直接替换到 body 模板中。因此，**变量的默认值必须与 body 模板中该字段的 JSON 类型匹配**，否则会产生无效 JSON。

### 9.1 字符串字段（后端需要 `"value"`）

当后端 payload 中字段为字符串类型时，body 模板中用**双引号包裹**变量引用，默认值设为 `""`。

```yaml
variables:
  - key: order_no
    value: ""                       # 字符串字段默认空字符串

# body 模板：双引号包裹
body: '{"orderNo":"${order_no}"}'

# IF-ELSE 条件判断
expr: '${order_no} != ""'
```

**替换结果**：

| 变量值 | 替换后 body | 是否有效 JSON |
|--------|------------|--------------|
| `order_no = "ORD123"` | `{"orderNo":"ORD123"}` | ✓ |
| `order_no = ""` | `{"orderNo":""}` | ✓ |

### 9.2 整数字段（后端需要 `123`）

当后端 payload 中字段为整数类型时，body 模板中**不加引号**直接引用变量，默认值必须设为 `"0"`（字符串形式的数字）。

```yaml
variables:
  - key: order_id
    value: "0"                      # 整数字段默认 "0"，不能是 ""

# body 模板：不加引号
body: '{"orderId":${order_id}}'

# IF-ELSE 条件判断
expr: '${order_id} != "0"'
```

**替换结果**：

| 变量值 | 替换后 body | 是否有效 JSON |
|--------|------------|--------------|
| `order_id = "12345"` | `{"orderId":12345}` | ✓ |
| `order_id = "0"` | `{"orderId":0}` | ✓ |
| `order_id = ""` | `{"orderId":}` | ✗ **无效 JSON！** |

### 9.3 常见错误与排查

**错误现象**：日志中出现 `body_preview: {"orderId":,"couponId":0}`，后端 JSON 反序列化失败。

**原因**：整数字段的默认值设为了 `""`，变量替换后 `${order_id}` 变成空字符串，导致 body 中出现 `orderId:` 后无值的无效 JSON。

**修复方法**：

```yaml
# ❌ 错误配置
variables:
  - key: order_id
    value: ""                       # 整数字段不能用空字符串
body: '{"orderId":${order_id}}'     # 替换后 → {"orderId":}

# ✅ 正确配置
variables:
  - key: order_id
    value: "0"                      # 整数字段默认 "0"
body: '{"orderId":${order_id}}'     # 替换后 → {"orderId":0}
```

### 9.4 速查表

| 后端字段类型 | Go struct 示例 | 默认值 | body 模板 | IF-ELSE 条件 |
|-------------|---------------|--------|-----------|-------------|
| `string` | `OrderNo string` | `""` | `'{"no":"${order_no}"}'` | `${order_no} != ""` |
| `int64` | `OrderId int64` | `"0"` | `'{"id":${order_id}}'` | `${order_id} != "0"` |
| `int64 omitempty` | `CouponId int64,omitempty` | `"0"` | `'{"cid":${coupon_id}}'` | `${coupon_id} != "0"` |

> **规则**：body 模板中变量**被引号包裹** → 默认 `""`；变量**不被引号包裹** → 默认 `"0"`。

---

## 10. 完整示例

以下是一个综合示例，涵盖主要节点类型：

```yaml
name: 综合示例场景
description: |
  演示 Salvo 主要节点类型和功能的综合场景。

variables:
  - key: base_url
    value: "http://localhost:9090/mock/api"
  - key: token
    value: ""
  - key: order_id
    value: ""
  - key: payment_status
    value: ""

config_params:
  env_type: "uat1"
  max_retry: "3"

data_sources:
  - name: users
    columns: [username, password]
    rows:
      - username: "admin@example.com"
        password: "admin123"

setup:
  - name: 登录
    type: setup
    config:
      method: POST
      url: "${base_url}/auth/login"
      headers: {Content-Type: application/json}
      body: '{"username":"${users.username}","password":"${users.password}"}'
      timeout: 5
      extract:
        token: "$.data.token"

nodes:
  # 生成器: 雪花ID
  - name: 生成订单号
    type: generator
    config:
      expression: '${__snowflakeId()}'
      variable: order_id

  # HTTP 请求
  - name: 创建订单
    type: http
    config:
      method: POST
      url: "${base_url}/orders"
      headers:
        Content-Type: application/json
        Authorization: "Bearer ${token}"
      body: '{"order_id":"${order_id}","amount":${__random(10, 500, 2)}}'
      timeout: 5
      extract:
        order_id: "$.data.id"

  # 延迟
  - name: 等待处理
    type: delay
    config:
      ms: 500

  # 定时器(延迟模式)
  - name: 支付后等待
    type: timer
    config:
      mode: delay
      seconds: 1

  # 条件分支
  - name: 判断是否支付
    type: if-else
    config:
      expr: '${order_id} != ""'

  - name: 支付
    type: http
    config:
      method: POST
      url: "${base_url}/payment"
      body: '{"order_id":"${order_id}"}'
      extract:
        payment_status: "$.data.status"

  - name: 跳过支付
    type: http
    config:
      method: GET
      url: "${base_url}/orders/${order_id}"

  # while 循环: 轮询支付状态
  - name: 轮询支付状态
    type: while
    config:
      exit_conditions:
        - variable: payment_status
          operator: equals
          value: "SUCCESS"
      interval_seconds: 2
      max_iterations: 10
      steps:
        - name: 查询状态
          request:
            method: GET
            url: "${base_url}/orders/${order_id}/status"
            headers: {Authorization: "Bearer ${token}"}
          extract:
            - variable: payment_status
              path: "$.data.status"

  # 并行查询
  - name: 并行查询
    type: parallel
    config:
      steps:
        - name: 查询用户
          request:
            method: GET
            url: "${base_url}/users"
          extract:
            - variable: user_count
              path: "$.data.total"
        - name: 查询商品
          request:
            method: GET
            url: "${base_url}/products"
          extract:
            - variable: product_count
              path: "$.data.total"

teardown:
  - name: 清理
    type: teardown
    config:
      method: DELETE
      url: "${base_url}/orders/${order_id}"
      headers: {Authorization: "Bearer ${token}"}

edges:
  - from: 登录
    to: 生成订单号
  - from: 生成订单号
    to: 创建订单
  - from: 创建订单
    to: 等待处理
  - from: 等待处理
    to: 判断是否支付
  - from: 判断是否支付
    to: 支付
    condition: "__if_true__"
  - from: 判断是否支付
    to: 跳过支付
    condition: "__if_false__"
  - from: 支付
    to: 轮询支付状态
  - from: 跳过支付
    to: 轮询支付状态
  - from: 轮询支付状态
    to: 并行查询
  - from: 并行查询
    to: 清理
```

---

## 附：节点类型速查表

| 类型 | 关键配置 | 典型场景 |
|------|----------|----------|
| `http` | `method,url,body,extract` | API 调用 |
| `setup` | 同 http | 初始化 |
| `teardown` | 同 http | 清理 |
| `generator` | `expression,variable` | 生成变量 |
| `delay` | `ms` | 固定延迟 |
| `timer` | `mode,seconds` | 延迟/间隔 |
| `condition` | `expr` | 条件求值 |
| `if-else` | `expr` + 边条件 | 二分支 |
| `group` | `node_ids,loop_count` | 子流程循环 |
| `while` | `exit_conditions,steps` | 条件循环 |
| `loop` | `loop_count,steps` | 定数循环 |
| `parallel` | `steps` | 并发执行 |
| `sub_flow` | `scene_id` | 嵌套场景 |
