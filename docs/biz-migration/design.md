# Salvo 迁移新增系统函数与表达式需求清单

> 基于 snailx 迁移 Salvo 方案设计文档及 5 个 YAML 流程文件整理

---

## 一、__weightedChoice 设计（Go 实现友好型）

### 1.1 设计原则

鉴于 Salvo 扩展函数基于 **Go 语言** 实现，Go 是静态类型语言，无法像 Python 那样在运行时动态区分 `list`/`dict` 参数类型。因此采用**单一字符串键值对参数**设计，由 Go 端统一解析，保持接口简洁。

### 1.2 函数签名

```
__weightedChoice(keyValuePairs)
```

- **参数**：`keyValuePairs` — 键值对字符串，格式为 `key1=weight1,key2=weight2,...`
- **返回值**：按权重随机选中的 `key`

### 1.3 Salvo 表达式示例

```
# 二选一（布尔型/开关型）
${__weightedChoice("1=50,0=50")}                         --> 50% 概率返回 1，50% 返回 0
${__weightedChoice("true=80,false=20")}                  --> 80% 概率返回 true，20% 返回 false

# 多选一（自定义权重）
${__weightedChoice("A=40,B=30,C=20,D=10")}             --> 按 40/30/20/10 权重返回 A/B/C/D
${__weightedChoice("10=50,20=30,30=20")}               --> 50%返回10, 30%返回20, 20%返回30

# 等概率多选一（权重均分）
${__weightedChoice("A=25,B=25,C=25,D=25")}             --> 等概率返回 A/B/C/D
```

### 1.4 YAML 配置层映射

```yaml
# 二选一
autoPay:
  weighted_choice:
    weights:
      1: 50
      0: 50

# 多选一（自定义权重）
chargeStrategy:
  weighted_choice:
    weights:
      A: 40
      B: 30
      C: 20
      D: 10

# 等概率多选一
commentStar:
  weighted_choice:
    weights:
      10: 20
      20: 20
      30: 20
      40: 20
      50: 20
```

### 1.5 权重校验策略（Go 实现核心逻辑）

当用户输入的权重总和 **不等于 100%** 时（例如 `{A=40, B=30, C=20}` 总和为 90），采用**归一化（Normalize）**策略：

```go
// Go 伪代码
func weightedChoice(kvPairs string) string {
    // 1. 解析键值对
    items := parseKeyValuePairs(kvPairs)  // [{key:"A", weight:40}, {key:"B", weight:30}, ...]

    // 2. 计算权重总和
    totalWeight := 0
    for _, item := range items {
        totalWeight += item.weight
    }

    // 3. 归一化：如果总和 != 100，按 totalWeight 比例缩放
    //    例如 {A=40, B=30, C=20} 总和=90
    //    实际概率: A=40/90≈44.4%, B=30/90≈33.3%, C=20/90≈22.2%

    // 4. 生成 [0, totalWeight) 范围内的随机数
    random := rand.Intn(totalWeight)

    // 5. 按权重区间匹配
    cumulative := 0
    for _, item := range items {
        cumulative += item.weight
        if random < cumulative {
            return item.key
        }
    }

    // 兜底返回最后一个
    return items[len(items)-1].key
}
```

**处理规则总结**：

| 权重和情况 | 处理方式 | 示例 | 实际概率分布 |
|-----------|--------|------|------------|
| **等于 100** | 直接按权重分配 | `A=40,B=30,C=20,D=10` | A=40%, B=30%, C=20%, D=10% |
| **小于 100** | 归一化后按比例分配 | `A=40,B=30,C=20` (和=90) | A=44.4%, B=33.3%, C=22.2% |
| **大于 100** | 归一化后按比例分配 | `A=50,B=50,C=50` (和=150) | A=33.3%, B=33.3%, C=33.3% |
| **存在权重 <= 0** | 过滤该选项（或报错） | `A=0,B=50` | 过滤A，B=100% |
| **只有一个选项** | 直接返回该选项 | `A=100` | 返回 A |

---

## 二、自定义系统函数

| 函数名 | 参数 | 返回值 | 对应 snailx 源码 | 说明 | example |
|--------|------|--------|------------------|------|---------|
| `__weightedChoice` | `(keyValuePairs)` — `key=weight,...` | 选中 key | `OneofWithProbability` | 概率加权选择，支持归一化 | `${__weightedChoice("1=50,0=50")}` → 50%概率返回1 |
| `__oneOf` | `(item1, item2, ...)` | 选中项 | `Oneof(list)` | 等概率选一个 | `${__oneOf("A","B","C")}` → "B" |
| `__manOf` | `(item1, item2, ...)` | 逗号分隔子集 | `Manyof` | 随机选1~N个子集 | `${__manOf(1,2,3,4,5,6,7)}` → "2,4,7" |
| `__Random` | `(min, max)` 或 `(min, max, scale)` | 整数或小数 | `Random(min,max)` | 范围 [min, max] **包含边界**；scale 为小数位数 | `${__Random(60,600)}` → 整数 123；`${__Random(1.5, 9.5, 2)}` → 小数 3.47 |
| `__snowflakeId` | `()` | 19位long字符串 | `Snowflake.NextId()` | 雪花ID | `${__snowflakeId()}` → "1234567890123456789" |

### __Random 详细说明

```
# 整数生成（包含边界 min 和 max）
${__Random(60, 600)}              --> 生成整数，范围 [60, 600]，包含 60 和 600
${__Random(0, 100)}               --> 生成整数，范围 [0, 100]

# 小数生成（包含边界，scale 指定小数位数）
${__Random(1.5, 9.5, 2)}         --> 生成小数，范围 [1.50, 9.50]，保留2位小数
${__Random(0.0, 1.0, 4)}         --> 生成小数，范围 [0.0000, 1.0000]，保留4位小数
```

**Go 实现方案（统一入口 + 内部独立函数）**：

由于 Go 泛型无法同时约束 `int` 和 `float64`（算术运算 API 不同），且反射在压测场景性能损耗大，因此采用**统一字符串入口 + 内部两个独立函数**的方案：

```go
// Salvo 统一入口：根据参数个数分发
func Random(args ...string) (string, error) {
    switch len(args) {
    case 2: // 整数: __Random(60, 600)
        min, _ := strconv.ParseInt(args[0], 10, 64)
        max, _ := strconv.ParseInt(args[1], 10, 64)
        return strconv.FormatInt(randomInt(min, max), 10), nil
    case 3: // 浮点数: __Random(1.5, 9.5, 2)
        min, _ := strconv.ParseFloat(args[0], 64)
        max, _ := strconv.ParseFloat(args[1], 64)
        scale, _ := strconv.Atoi(args[2])
        return fmt.Sprintf("%.*f", scale, randomFloat(min, max, scale)), nil
    default:
        return "", errors.New("__Random expects 2 args (int) or 3 args (float, scale)")
    }
}

// 内部函数1：生成 [min, max] 范围内的整数，包含边界
func randomInt(min, max int64) int64 {
    if min >= max {
        return min
    }
    return min + rand.Int63n(max-min+1)  // rand.Int63n(n) 返回 [0, n)，所以 +1 保证包含 max
}

// 内部函数2：生成 [min, max] 范围内的浮点数，包含边界，保留 scale 位小数
func randomFloat(min, max float64, scale int) float64 {
    if min >= max {
        return min
    }
    raw := min + rand.Float64()*(max-min)  // rand.Float64() 返回 [0.0, 1.0)，乘后范围 [min, max)
    factor := math.Pow(10, float64(scale))
    return math.Round(raw*factor) / factor   // 四舍五入保留 scale 位
}
```

**设计要点**：

| 层面 | 方案 | 说明 |
|------|------|------|
| **对外接口** | `__Random(min, max)` 或 `__Random(min, max, scale)` | 统一函数名，参数个数区分 |
| **内部实现** | `randomInt()` + `randomFloat()` 两个独立函数 | 零反射、零泛型、类型安全、性能最优 |
| **边界处理** | `[min, max]` 均**包含边界** | 整数通过 `+1` 实现，浮点通过 `Float64()` 天然特性实现 |

---

## 三、YAML 参数生成模式

YAML 配置层的 `config_params` 和 `derived_params` 支持以下模式，底层调用第二节的系统函数：

| 模式 | YAML 关键字 | 底层调用 | 说明 | example |
|------|-------------|----------|------|---------|
| 固定值 | `values: [0]` | — | 直接固定值，或等概率选一个 | `orderSource: [0]` → 固定 0 |
| 区间随机 | `range: {min, max}` | `__Random` | 生成整数，范围 [min, max] | `chargeTime: {min: 60, max: 600}` |
| 概率加权 | `weighted_choice: {weights: {}}` | `__weightedChoice` | 按权重概率选择 | `autoPay: {weights: {1:50, 0:50}}` |
| 多选子集 | `function: manOf` + `source: [...]` | `__manOf` | 随机选 1~N 个子集 | `tags: {function: manOf, source: [1,2,3,4,5,6,7]}` |
| 函数调用 | `function: xxx` | 对应系统函数 | 调用 snowflakeId 等 | `subTaskId: {function: snowflakeId}` |
| 衍生计算 | `expression: "..."` | 表达式引擎 | 支持变量引用和数学运算 | `expression: "${chargeTime} * ${ranking} / 100"` |

### YAML 与系统函数的映射关系

```yaml
# 固定值
orderSource:
  values: [0]                      # 直接固定

# 等概率选择（底层调用 __oneOf）
commentStar:
  values: [10, 20, 30, 40, 50]     # 等概率选一个，底层 __oneOf(10,20,30,40,50)

# 区间随机（底层调用 __Random）
chargeTime:
  range: { min: 60, max: 600 }     # 底层 __Random(60, 600)

# 概率加权（底层调用 __weightedChoice）
autoPay:
  weighted_choice:
    weights:
      1: 50
      0: 50                        # 底层 __weightedChoice("1=50,0=50")

# 多选子集（底层调用 __manOf）
tags:
  function: manOf
  source: [1, 2, 3, 4, 5, 6, 7]   # 底层 __manOf(1,2,3,4,5,6,7)

# 函数调用（直接调用系统函数）
subTaskId:
  function: snowflakeId            # 底层 __snowflakeId()

# 衍生计算（表达式引擎）
chargePostOfflineTime:
  expression: "${chargeTime} * ${chargePostOfflineTimeRanking} / 100"
```

---

## 四、条件判断运算符

| 运算符 | 说明 | 使用场景 | example |
|--------|------|----------|---------|
| `equals` | 等于判断 | `forceStopCharge == 1` | `{variable: forceStopCharge, operator: equals, value: 1}` |
| `not_equals` | 不等于判断 | `orderStatus != "COMPLETED"` | `{variable: orderStatus, operator: not_equals, value: "COMPLETED"}` |
| `greater_than` | 大于判断 | `preOccupyTime > 0` | `{variable: preOccupyTime, operator: greater_than, value: 0}` |
| `greater_than_or_equal` | 大于等于判断 | `count >= 1` | `{variable: count, operator: greater_than_or_equal, value: 1}` |
| `less_than` | 小于判断 | `count < 10` | `{variable: count, operator: less_than, value: 10}` |
| `less_than_or_equal` | 小于等于判断 | `count <= 5` | `{variable: count, operator: less_than_or_equal, value: 5}` |
| `not_empty` | 非空判断 | `unpaidOrderId` 有值 | `{variable: unpaidOrderId, operator: not_empty}` |
| `empty` | 为空判断 | `occupyOrderId` 为空 | `{variable: occupyOrderId, operator: empty}` |
| `size_equals` | 集合大小等于 | `orders.size == 1` | `{variable: unpaidOrders, operator: size_equals, value: 1}` |
| `size_greater_than` | 集合大小大于 | `orders.size > 1` | `{variable: inproOrders, operator: size_greater_than, value: 1}` |
| `size_greater_than_or_equal` | 集合大小大于等于 | `orders.size >= 1` | `{variable: occupyOrders, operator: size_greater_than_or_equal, value: 1}` |
| `size_less_than` | 集合大小小于 | `orders.size < 5` | `{variable: orders, operator: size_less_than, value: 5}` |

---

## 五、步骤类型与控制关键字

| 关键字/类型 | 说明 | 使用场景 | example |
|------------|------|----------|---------|
| `type: while` | While 循环控制器 | 轮询充电状态、轮询占位订单 | 见下方 YAML 示例 |
| `type: parallel` | 并行请求组 | miniIndexPage 首页初始化、并行 API | 见下方 YAML 示例 |
| `type: sub_flow` | 子流程 | 个人中心、登录后首页等子流程 | `type: sub_flow` + `async: true` |
| `async: true` | 异步执行标记 | sub_flow 异步派生 | 配合 `sub_flow` 使用 |
| `interval_seconds` | While 循环间隔秒数 | 轮询间隔 30 秒 | `interval_seconds: 30` |
| `max_iterations` | While 最大迭代次数 | 占位订单轮询最多 6 次 | `max_iterations: 6` |
| `max_duration_minutes` | While 最大持续时长(分钟) | 占位订单轮询 5 分钟超时 | `max_duration_minutes: 5` |
| `exit_conditions` | While 退出条件列表 | `chargingStatus == "4"` | 见下方 YAML 示例 |
| `fail_after_consecutive` | 连续失败阈值 | 启动超时连续 10 次失败 | `fail_after_consecutive: 10` |
| `fail_message` | 连续失败提示信息 | "启动超时(连续10次启动中)" | `fail_message: "启动超时"` |
| `timed_trigger` | 时间触发器(循环内单次/多次) | 桩离线/上线、故障码(基于 chargeTime 计算) | 见下方 YAML 示例 |
| `once: true` | 时间触发器仅执行一次 | 配合 `timed_trigger` 使用 | `once: true` |
| `retry` | 请求重试配置 | 停止充电重试 10 次(应对 429 限流) | `retry: {max_attempts: 10, on_429: retry}` |
| `think_time` | 随机思考时间(毫秒) | 模拟用户页面停留 | `think_time: {min: 0, max: 20000}` |
| `condition` | 步骤执行条件 | 概率性步骤、条件性请求 | `condition: {variable, operator, value}` |

### While 循环完整示例

```yaml
- name: 轮询充电状态
  type: while
  exit_conditions:
    - variable: chargingStatus
      operator: equals
      value: "4"
  interval_seconds: 30
  steps:
    - name: 查询充电状态
      request:
        url: "${BASE_URL}${API_CHARGE_STATUS}"
        method: POST
        headers: { Authorization: "${token}" }
        body: { startChargeSeq: "${startChargeSeq}" }
      extract:
        - chargingStatus: $.result.status
    - name: 启动超时检查
      condition: { variable: chargingStatus, operator: equals, value: "1" }
      fail_after_consecutive: 10
      fail_message: "启动超时(连续10次启动中)"
    - name: 桩离线(时间触发)
      condition: { variable: chargePostOffline, operator: equals, value: 1 }
      timed_trigger: { after_seconds: "${chargePostOfflineTime}", once: true }
      request:
        url: "${SIM_URL}${API_SET_STATION_STATUS}"
        method: POST
        body: { simAddr: "${simAddr}", simPort: "${simPort}", stationStatus: 2 }
```

### Parallel 并行请求完整示例

```yaml
- name: 首页初始化并行请求
  type: parallel
  steps:
    - name: 获取前端配置
      request:
        url: "${BASE_URL}${API_GET_FRONT_CONFIG}"
        method: GET
    - name: 查询广告位1
      request:
        url: "${BASE_URL}${API_QUERY_ADV_SPACE}"
        method: POST
        body: { position: [1], cityCode: "${cityCode}" }
```

---

## 六、表达式计算支持

| 表达式类型 | 示例 | 说明 | example |
|------------|------|------|---------|
| 变量引用 | `${chargeTime}` | 引用 config_params 或 extract 的变量 | `${token}` → 引用提取的 token |
| 数学运算 | `${chargeTime} * ${ranking} / 100` | 支持加减乘除 | `60 * 50 / 100` → 30 |
| 变量混合运算 | `${chargePostOfflineTime + 300}` | 变量与常量混合计算 | `120 + 300` → 420 |
| 嵌套函数调用 | `${__Random(${cities.lat_min}, ${cities.lat_max})}` | 函数参数可嵌套变量 | 随机生成 GPS 坐标 |
| 数组索引访问 | `${unpaidChargeOrders[0].orderId}` | 列表/数组元素属性访问 | 取第一个订单的 orderId |
| 字符串拼接 | `"用户:${USERS.phone}, 订单号:${orderId}"` | 模板字符串拼接 | `"Hello, ${name}!"` |

---

## 七、特殊机制

| 机制 | 说明 | 实现方式 | example |
|------|------|----------|---------|
| 签名机制 | RSA+SHA256 签名 body | `__groovy(signIfNeeded(body, signApiList, rsaPriKey))` | 见下方签名示例 |
| 关联数据源 | 城市→GPS 行级关联 | CSV 数据源读取时随机选行，同列绑定 | `cities.city_code` → "440100" |
| 概率性条件 | 条件性步骤执行 | `condition: {variable, operator, value}` | 见上方 condition 示例 |
| 集合条件判断 | 基于列表大小的条件分支 | `size_equals`/`size_greater_than` 等 | `{variable: orders, operator: size_greater_than, value: 1}` |
| 变量作用域 | 全局变量 vs 步骤局部变量 | 全局变量 `${VAR}`，步骤变量通过 `extract` 定义 | `extract: [{token: $.result.token}]` |

### 签名机制完整示例

```yaml
- name: 获取签名API列表
  request:
    url: "${BASE_URL}${API_GET_SIGN_API_LIST}"
    method: GET
  extract:
    - signApiList: $.result

- name: 业务请求(含签名)
  request:
    url: "${BASE_URL}${API_XXX}"
    headers:
      Authorization: "${token}"
      Signature: "${__groovy(signIfNeeded(body, signApiList, rsaPriKey))}"
```

---

## 八、YAML 配置完整示例

### personal_card_charge.yaml 数据生成层示例

```yaml
data_generation:
  data_sources:
    users:
      type: csv
      file: "${USER_DATA_SOURCE}"
      columns: [phone, userid, cardnum, b2b_userid, b2b_cardnum, b2b_cardpwd, user_vip, safe_buy_flag]
    chargers:
      type: csv
      file: "${CHARGER_DATA_SOURCE}"
      columns: [charger, pointId, equipNo, connectorId, stationId, power, city_code]

  config_params:
    subTaskId:
      function: snowflakeId
    ifPrepare:
      values: [1, 2]
    orderSource:
      values: [0]
    payType:
      values: [1]
    payChannel:
      values: [2]
    preOccupyTime:
      range: { min: "${CHARGE_PREOCCUPY_MIN}", max: "${CHARGE_PREOCCUPY_MAX}" }
    afterOccupyTime:
      range: { min: "${CHARGE_AFTEROCCUPY_MIN}", max: "${CHARGE_AFTEROCCUPY_MAX}" }
    chargeTime:
      range: { min: "${CARD_CHARGE_TIME_MIN}", max: "${CARD_CHARGE_TIME_MAX}" }
    autoPay:
      weighted_choice:
        weights:
          1: "${CARD_AUTO_PAY_PROBABILITY}"
          0: "${CARD_AUTO_PAY_INVERSE_PROBABILITY}"
    forceStopCharge:
      weighted_choice:
        weights:
          1: "${CARD_FORCE_STOP_PROBABILITY}"
          0: "${CARD_FORCE_STOP_INVERSE_PROBABILITY}"
    chargePostOffline:
      weighted_choice:
        weights:
          1: "${CARD_CHARGE_POST_OFFLINE_PROBABILITY}"
          0: "${CARD_CHARGE_POST_OFFLINE_INVERSE_PROBABILITY}"
    raiseErrorCode:
      weighted_choice:
        weights:
          1: "${CARD_RAISE_ERROR_PROBABILITY}"
          0: "${CARD_RAISE_ERROR_INVERSE_PROBABILITY}"
    minChargeTime:
      range: { min: "${MIN_CHARGE_TIME_MIN}", max: "${MIN_CHARGE_TIME_MAX}" }
    minChargePower:
      range: { min: "${MIN_CHARGE_POWER_MIN}", max: "${MIN_CHARGE_POWER_MAX}" }
    maxChargePower:
      range: { min: "${MAX_CHARGE_POWER_MIN}", max: "${MAX_CHARGE_POWER_MAX}" }
    maxChargePowerKw:
      expression: "${maxChargePower} / 1000"
    minSoc:
      range: { min: "${MIN_SOC_MIN}", max: "${MIN_SOC_MAX}" }
    maxSoc:
      range: { min: "${MAX_SOC_MIN}", max: "${MAX_SOC_MAX}" }
    singleMaxPower:
      range: { min: "${SINGLE_MAX_POWER_MIN}", max: "${SINGLE_MAX_POWER_MAX}" }
    meterSamplePeriod:
      range: { min: "${METER_SAMPLE_PERIOD_MIN}", max: "${METER_SAMPLE_PERIOD_MAX}" }
    raiseErrorCodeVal:
      values: [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50]
    safeBuyFlag:
      values: [0, 1]
    userVip:
      values: [true, false]

  derived_params:
    - name: chargePostOfflineTimeRanking
      range: { min: "${CHARGE_POST_OFFLINE_TIME_RANK_MIN}", max: "${CHARGE_POST_OFFLINE_TIME_RANK_MAX}" }
    - name: chargePostOfflineTime
      expression: "${chargeTime} * ${chargePostOfflineTimeRanking} / 100"
    - name: chargePostOnlineTime
      range: { min: "${chargePostOfflineTime}", max: "${chargePostOfflineTime + 300}" }
    - name: raiseErrTimeRanking
      range: { min: "${RAISE_ERR_TIME_RANK_MIN}", max: "${RAISE_ERR_TIME_RANK_MAX}" }
    - name: raiseErrorTime
      expression: "${chargeTime} * ${raiseErrTimeRanking} / 100"
    - name: commentStar
      values: [10, 20, 30, 40, 50]
    - name: tags
      function: manOf
      source: [1, 2, 3, 4, 5, 6, 7]
```

### mini_app_full_journey.yaml 数据生成层示例（城市→GPS关联）

```yaml
data_generation:
  data_sources:
    users:
      type: csv
      file: "${USER_DATA_SOURCE}"
      columns: [phone, userid]
    cities:
      type: csv
      file: "${CITY_GPS_DATA_SOURCE}"
      columns: [city_code, city_name, lat_min, lat_max, lon_min, lon_max]

  config_params:
    subTaskId:
      function: snowflakeId
    order:
      values: [0, 1, 2, 3]
    ifLogin:
      weighted_choice:
        weights:
          1: "${IF_LOGIN_PROBABILITY}"
          0: "${IF_LOGIN_INVERSE_PROBABILITY}"
    switchCityCode:
      weighted_choice:
        weights:
          "__oneOf('110100','310100','440100')": "${SWITCH_CITY_PROBABILITY}"
          "": "${SWITCH_CITY_INVERSE_PROBABILITY}"
    searchKeyword:
      values: ["特来电", "星星充电", "国家电网", "南方电网", "快充", "慢充", "免费停车"]

  derived_params:
    # 城市 → GPS坐标（CSV 数据源读取时随机选行，同列绑定）
    - name: cityCode
      expression: "${cities.city_code}"
    - name: lat
      expression: "${__Random(${cities.lat_min}, ${cities.lat_max})}"
    - name: lon
      expression: "${__Random(${cities.lon_min}, ${cities.lon_max})}"

    # MoreCondition 筛选条件(6个多选子集)
    - name: chargingTypeSet
      function: manOf
      source: [1, 2, 3, 4]
    - name: openSet
      function: manOf
      source: [1, 2, 3]
    - name: parkFreeFlagSet
      function: manOf
      source: [1, 2, 3]
    - name: parkLocationSet
      function: manOf
      source: [1, 2, 3, 4]
    - name: recommendSet
      function: manOf
      source: [1, 2]
    - name: tagSet
      function: manOf
      source: [1, 2, 3, 4, 5, 6, 7]
```

---

## 九、函数兼容性说明

| snailx 原函数 | Salvo 函数 | 参数差异 | 备注 |
|---------------|-----------|----------|------|
| `Random(min,max)` | `__Random(min,max)` | 无差异，[min, max] 包含边界 | 直接映射 |
| `Oneof(list)` | `__oneOf(...)` | 无差异 | 直接映射 |
| `OneofWithProbability(p,[a,b])` | `__weightedChoice("a=p,b=100-p")` | 单一字符串参数，归一化计算 | 直接映射，新增多值扩展 |
| `Manyof(list)` | `__manOf(...)` | 无差异 | 直接映射 |
| `Snowflake.NextId()` | `__snowflakeId()` | 无差异 | 直接映射 |
| `GenSenquence/GetStep` | — | 不需要 | Salvo 线程调度替代 |
| 城市→GPS关联 | CSV 数据源行级绑定 | 去掉 `__randomFromDataset` | CSV 读取时随机选行，同列自动绑定 |
