# P4 参数生成器模块

## 架构总览

```
JSON Schema / OpenAPI / 手动配置
        │
        ▼
   Schema (归一化内部表示)
        │
        ▼
   Registry → 匹配 Generator → Generate() → 值
```

三种 Schema 来源统一归一化为 `generator.Schema` 内部表示，再由 `Registry` 分派到匹配的 `Generator` 生成值。

## 文件结构

```
internal/generator/
├── generator.go          # 核心接口：Schema、Generator、Registry
├── errors.go             # 错误常量
├── generator_test.go     # Registry + 接口测试
├── schema/
│   ├── schema.go         # JSON Schema Draft 7 解析器 + OpenAPI/Swagger 解析
│   └── schema_test.go    # 解析器测试
└── builtin/
    ├── builtin.go        # 13 个内置生成器 + DefaultRegistry 工厂
    └── builtin_test.go   # 生成器测试
```

## 核心接口

### Schema

`Schema` 是 JSON Schema Draft 7 的归一化内部表示，覆盖所有影响值生成的关键字：

```go
type Schema struct {
    Type       Type       // string | number | integer | boolean | array | object | null
    Enum       []any      // 枚举值
    HasConst   bool       // 是否有 const 约束
    ConstVal   any        // const 值
    HasDefault bool       // 是否有默认值
    DefaultVal any        // 默认值

    // string 约束
    MinLength  *int
    MaxLength  *int
    Pattern    string
    Format     string     // uuid | email | date | date-time | ipv4 | ipv6 | ...

    // number/integer 约束
    Minimum    *float64
    Maximum    *float64
    ExclMin    *float64   // exclusiveMinimum
    ExclMax    *float64   // exclusiveMaximum
    MultipleOf *float64

    // array 约束
    MinItems   *int
    MaxItems   *int
    Unique     bool       // uniqueItems
    Items      *Schema    // 元素 schema

    // object 约束
    Properties map[string]*Schema
    Required   []string
    AddlProps  *bool      // additionalProperties

    // 组合关键字
    AllOf      []*Schema
    AnyOf      []*Schema
    OneOf      []*Schema

    Title       string
    Description string
}
```

### Generator

```go
type Generator interface {
    Generate(schema *Schema) (any, error)  // 生成符合 schema 的值
    CanHandle(schema *Schema) bool         // 判断是否能处理该 schema
    Name() string                          // 生成器标识
}
```

### Registry

```go
type Registry struct { ... }

func (r *Registry) Register(g Generator)       // 注册生成器（先注册优先）
func (r *Registry) Generate(schema *Schema) (any, error)  // 分派生成
```

`Registry.Generate` 的调度逻辑：

1. `schema == nil` → 返回 `nil`
2. `HasConst` → 返回 `ConstVal`
3. `HasDefault` → 返回 `DefaultVal`
4. `Enum` 非空 → 返回第一个枚举值
5. 遍历已注册 Generator，首个 `CanHandle == true` 的执行 `Generate`
6. 无匹配 → 返回 `ErrNoGenerator`

## 内置生成器

按 `DefaultRegistry()` 注册优先级排列（先注册先匹配）：

| 生成器 | Name | Schema 匹配条件 | 说明 |
|--------|------|-----------------|------|
| UUIDGenerator | `uuid` | `string + format=uuid` | UUID v4，使用 crypto/rand |
| EmailGenerator | `email` | `string + format=email` | 随机邮箱，可自定义域名 |
| DateGenerator | `date` | `string + format=date` | 日期字符串 `2006-01-02` |
| DateTimeGenerator | `date-time` | `string + format=date-time` | UTC 时间 RFC3339 |
| FormatStringGenerator | `format-string` | `string + format=*` | ipv4/ipv6/hostname/uri/url/byte/password |
| EnumString | `enum-string` | `enum` 非空 | 从枚举值中随机选取 |
| RandomString | `random-string` | `string`（无 pattern/format） | 随机字母数字，支持 minLength/maxLength |
| RandomInt | `random-int` | `integer` | 随机整数，支持 minimum/maximum/exclusive* |
| RandomFloat | `random-float` | `number` | 随机浮点数，支持 multipleOf |
| RandomBool | `random-bool` | `boolean` | 50/50 随机 |
| NullGenerator | `null` | `null` | 返回 nil |
| ArrayGenerator | `array` | `array` | 递归生成 items，支持 uniqueItems |
| ObjectGenerator | `object` | `object` | 递归生成 properties |

额外生成器（需手动注册）：

| 生成器 | Name | 说明 |
|--------|------|------|
| IncrementInt | `increment-int` | 顺序递增整数，构造时指定起始值 |
| WeightedBool | `weighted-bool` | 可配置 true 比例的布尔值 |

## 使用示例

### 基本用法

```go
r := builtin.DefaultRegistry()

s := &generator.Schema{Type: generator.TypeString, Format: "uuid"}
val, err := r.Generate(s)
// val = "a1b2c3d4-e5f6-4789-a012-3456789abcde"
```

### 从 JSON Schema 生成

```go
data := []byte(`{
    "type": "object",
    "properties": {
        "id":    {"type": "string", "format": "uuid"},
        "name":  {"type": "string"},
        "age":   {"type": "integer", "minimum": 18, "maximum": 65},
        "email": {"type": "string", "format": "email"}
    },
    "required": ["id", "name"]
}`)

s, _ := schema.ParseBytes(data)
r := builtin.DefaultRegistry()
val, _ := r.Generate(s)
// val = map[string]any{
//     "id":    "550e8400-e29b-41d4-a716-446655440000",
//     "name":  "aB3kL9mN",
//     "age":   42,
//     "email": "xY7pQ2wM@example.com",
// }
```

### 从 OpenAPI 规范生成

```go
f, _ := os.Open("openapi.json")
schemas, _ := schema.ParseOpenAPI(f)
r := builtin.DefaultRegistry()
val, _ := r.Generate(schemas["User"])
```

### 自定义生成器

```go
type PhoneGenerator struct{}

func (g *PhoneGenerator) Name() string { return "phone" }
func (g *PhoneGenerator) CanHandle(s *generator.Schema) bool {
    return s.Type == generator.TypeString && s.Format == "phone"
}
func (g *PhoneGenerator) Generate(_ *generator.Schema) (any, error) {
    return "+1-" + randomDigits(3) + "-" + randomDigits(7), nil
}

r := builtin.DefaultRegistry()
r.Register(&PhoneGenerator{})  // 注册在 UUID/Email 之后
```

### 顺序递增 ID

```go
r := generator.NewRegistry()
r.Register(builtin.NewIncrementInt(1))  // 从 1 开始递增

s := &generator.Schema{Type: generator.TypeInteger}
val1, _ := r.Generate(s)  // 1
val2, _ := r.Generate(s)  // 2
val3, _ := r.Generate(s)  // 3
```

## JSON Schema Draft 7 支持的关键字

| 关键字 | 类型 | 状态 |
|--------|------|------|
| type | 全部 | ✅ |
| enum | 全部 | ✅ |
| const | 全部 | ✅ |
| default | 全部 | ✅ |
| minLength | string | ✅ |
| maxLength | string | ✅ |
| pattern | string | ⚠️ 解析支持，生成暂不支持正则引擎 |
| format | string | ✅ uuid/email/date/date-time/ipv4/ipv6/hostname/uri/url |
| minimum | number/integer | ✅ |
| maximum | number/integer | ✅ |
| exclusiveMinimum | number/integer | ✅ |
| exclusiveMaximum | number/integer | ✅ |
| multipleOf | number/integer | ✅ |
| minItems | array | ✅ |
| maxItems | array | ✅ |
| uniqueItems | array | ✅ |
| items | array | ✅ |
| properties | object | ✅ |
| required | object | ✅ |
| additionalProperties | object | ✅ |
| allOf | 组合 | ⚠️ 解析支持，生成待实现合并逻辑 |
| anyOf | 组合 | ⚠️ 解析支持，生成待实现随机选择 |
| oneOf | 组合 | ⚠️ 解析支持，生成待实现随机选择 |

## 测试覆盖率

| 包 | 覆盖率 |
|----|--------|
| `generator` | 100.0% |
| `generator/builtin` | 89.0% |
| `generator/schema` | 82.4% |

## 设计决策：为什么 Schema 采用扁平结构而非按类型拆分

### 问题

`Schema` 是一个大而全的扁平结构体，integer 类型的字段（如 `Minimum`、`Maximum`）和 string 类型的字段（如 `MinLength`、`Pattern`）混在一起。另一种设计是为不同数据类型设计不同的 Schema 结构体。哪种更好？

### 方案对比

**方案 A：扁平 Schema（当前设计）**

```go
type Schema struct {
    Type      Type
    MinLength *int       // string 专用
    Minimum   *float64   // number/integer 专用
    Properties map[string]*Schema  // object 专用
    // ...
}
```

**方案 B：按类型拆分 Schema**

```go
type Schema interface { Type() Type }

type StringSchema struct {
    MinLength *int
    Pattern   string
}

type IntegerSchema struct {
    Minimum    *float64
    Maximum    *float64
}

type ObjectSchema struct {
    Properties map[string]Schema  // 仍需接口
}
```

### 分析

| 维度 | 扁平 Schema | 按类型拆分 |
|------|------------|-----------|
| JSON 解析 | 直接反序列化 | 需两阶段：先解析 Type，再分派 |
| 编译期安全 | ❌ 无 | ✅ 有（但嵌套处失效） |
| 代码量 | 少 | 多（N 个 struct + 接口） |
| 扩展性 | 加字段即可 | 加 struct + 改接口 |
| 行业实践 | JSON Schema 库主流做法 | 少见 |

### 结论：选择扁平 Schema

1. **JSON Schema 本身就是扁平结构**——规范里 `type` 只是一个鉴别字段，不是类型系统
2. **嵌套递归是核心需求**——`ObjectSchema.Properties` 的值可以是任意类型，拆分后这里仍需接口，类型安全优势在嵌套处消失
3. **"无效字段"问题在实践中不严重**——Generator 的 `CanHandle` 已做运行时校验，`nil` 指针字段零开销
4. **主流 JSON Schema 库**（如 Go 的 `github.com/invopop/jsonschema`、Python 的 `jsonschema`）都采用扁平结构

如果未来需要编译期安全，可考虑混合方案：保持扁平 Schema 用于解析和存储，在 Generator 内部提供类型化辅助方法：

```go
func (s *Schema) StringConstraints() (minLen, maxLen *int, pattern, format string) {
    // 仅当 Type == TypeString 时调用
}
```

## 未来扩展

### 短期

- **正则生成器**：集成 `regen` 或 `rxgen` 库，支持 `pattern` 关键字的值生成
- **allOf/anyOf/oneOf 生成**：实现合并逻辑（allOf）和随机选择逻辑（anyOf/oneOf）
- **Lua 自定义生成器**：通过 Lua 脚本实现 `Generator` 接口

### 中期

- **引用解析**：支持 `$ref` 引用，自动展开并缓存
- **依赖生成器**：字段间关联（如 `endDate > startDate`）
- **Faker 集成**：更丰富的本地化数据（姓名、地址、公司名等）

### 长期

- **Schema 演化**：支持 JSON Schema Draft 2020-12
- **约束求解**：对复杂约束（如 `multipleOf` + `exclusiveMinimum` + `maxLength`）使用约束求解器
- **生成策略配置**：边界值策略（最小/最大/随机）、确定性种子（可复现测试）
