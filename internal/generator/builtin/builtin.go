// Package builtin provides built-in parameter generators for all
// JSON Schema types and common format strings.
package builtin

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/generator"
)

const (
	charsetAlpha  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	charsetAlnum  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetDigits = "0123456789"
	charsetHex    = "0123456789abcdef"
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

// --- UUIDGenerator ---

type UUIDGenerator struct{}

func (g *UUIDGenerator) Name() string { return "uuid" }

func (g *UUIDGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Format == "uuid"
}

func (g *UUIDGenerator) Generate(_ *generator.Schema) (any, error) {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		return nil, fmt.Errorf("uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// --- EmailGenerator ---

type EmailGenerator struct {
	domains []string
}

func NewEmailGenerator(domains ...string) *EmailGenerator {
	if len(domains) == 0 {
		domains = []string{"example.com", "test.org", "mail.io"}
	}
	return &EmailGenerator{domains: domains}
}

func (g *EmailGenerator) Name() string { return "email" }

func (g *EmailGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Format == "email"
}

func (g *EmailGenerator) Generate(_ *generator.Schema) (any, error) {
	user := randomString(charsetAlnum, 8)
	domains := g.domains
	if len(domains) == 0 {
		domains = []string{"example.com", "test.org", "mail.io"}
	}
	domain := domains[rand.Intn(len(domains))]
	return user + "@" + domain, nil
}

// --- DateGenerator ---

type DateGenerator struct{}

func (g *DateGenerator) Name() string { return "date" }

func (g *DateGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Format == "date"
}

func (g *DateGenerator) Generate(_ *generator.Schema) (any, error) {
	now := time.Now()
	offset := rand.Intn(365*5) - 365*2
	t := now.AddDate(0, 0, offset)
	return t.Format("2006-01-02"), nil
}

// --- DateTimeGenerator ---

type DateTimeGenerator struct{}

func (g *DateTimeGenerator) Name() string { return "date-time" }

func (g *DateTimeGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Format == "date-time"
}

func (g *DateTimeGenerator) Generate(_ *generator.Schema) (any, error) {
	now := time.Now().UTC()
	offset := rand.Intn(365*5) - 365*2
	t := now.AddDate(0, 0, offset)
	return t.Format(time.RFC3339), nil
}

// --- RandomString ---

type RandomString struct{}

func (g *RandomString) Name() string { return "random-string" }

func (g *RandomString) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Pattern == "" && s.Format == ""
}

func (g *RandomString) Generate(s *generator.Schema) (any, error) {
	minLen := 8
	maxLen := 16
	if s.MinLength != nil {
		minLen = *s.MinLength
	}
	if s.MaxLength != nil {
		maxLen = *s.MaxLength
	}
	if minLen > maxLen {
		return nil, generator.ErrInvalidRange
	}
	length := minLen
	if maxLen > minLen {
		length += rand.Intn(maxLen - minLen + 1)
	}
	return randomString(charsetAlnum, length), nil
}

// --- EnumString ---

type EnumString struct{}

func (g *EnumString) Name() string { return "enum-string" }

func (g *EnumString) CanHandle(s *generator.Schema) bool {
	return len(s.Enum) > 0
}

func (g *EnumString) Generate(s *generator.Schema) (any, error) {
	if len(s.Enum) == 0 {
		return nil, generator.ErrEmptyEnum
	}
	return s.Enum[rand.Intn(len(s.Enum))], nil
}

// --- RandomInt ---

type RandomInt struct{}

func (g *RandomInt) Name() string { return "random-int" }

func (g *RandomInt) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeInteger
}

func (g *RandomInt) Generate(s *generator.Schema) (any, error) {
	minVal, maxVal := effectiveIntRange(s)
	if minVal > maxVal {
		return nil, generator.ErrInvalidRange
	}
	return minVal + rand.Int63n(maxVal-minVal+1), nil
}

// --- IncrementInt ---

type IncrementInt struct {
	counter int64
}

func NewIncrementInt(start int64) *IncrementInt {
	return &IncrementInt{counter: start}
}

func (g *IncrementInt) Name() string { return "increment-int" }

func (g *IncrementInt) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeInteger
}

func (g *IncrementInt) Generate(_ *generator.Schema) (any, error) {
	val := g.counter
	g.counter++
	return val, nil
}

// --- RandomFloat ---

type RandomFloat struct{}

func (g *RandomFloat) Name() string { return "random-float" }

func (g *RandomFloat) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeNumber
}

func (g *RandomFloat) Generate(s *generator.Schema) (any, error) {
	minVal, maxVal := effectiveFloatRange(s)
	if minVal > maxVal {
		return nil, generator.ErrInvalidRange
	}
	val := minVal + rand.Float64()*(maxVal-minVal)
	if s.MultipleOf != nil && *s.MultipleOf > 0 {
		m := *s.MultipleOf
		steps := int(val / m)
		val = float64(steps) * m
		if val < minVal {
			val += m
		}
	}
	return val, nil
}

// --- RandomBool ---

type RandomBool struct{}

func (g *RandomBool) Name() string { return "random-bool" }

func (g *RandomBool) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeBoolean
}

func (g *RandomBool) Generate(_ *generator.Schema) (any, error) {
	return rand.Intn(2) == 1, nil
}

// --- WeightedBool ---

type WeightedBool struct {
	TrueWeight float64
}

func NewWeightedBool(trueWeight float64) *WeightedBool {
	return &WeightedBool{TrueWeight: trueWeight}
}

func (g *WeightedBool) Name() string { return "weighted-bool" }

func (g *WeightedBool) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeBoolean
}

func (g *WeightedBool) Generate(_ *generator.Schema) (any, error) {
	w := g.TrueWeight
	if w <= 0 {
		w = 0.5
	}
	if w > 1 {
		w = 1
	}
	return rand.Float64() < w, nil
}

// --- NullGenerator ---

type NullGenerator struct{}

func (g *NullGenerator) Name() string { return "null" }

func (g *NullGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeNull
}

func (g *NullGenerator) Generate(_ *generator.Schema) (any, error) {
	return nil, nil
}

// --- ArrayGenerator ---

type ArrayGenerator struct {
	Registry *generator.Registry
}

func (g *ArrayGenerator) Name() string { return "array" }

func (g *ArrayGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeArray
}

func (g *ArrayGenerator) Generate(s *generator.Schema) (any, error) {
	minItems := 0
	maxItems := 5
	if s.MinItems != nil {
		minItems = *s.MinItems
	}
	if s.MaxItems != nil {
		maxItems = *s.MaxItems
	}
	if minItems > maxItems {
		return nil, generator.ErrInvalidRange
	}

	count := minItems
	if maxItems > minItems {
		count += rand.Intn(maxItems - minItems + 1)
	}

	if s.Items == nil {
		arr := make([]any, count)
		return arr, nil
	}

	arr := make([]any, count)
	for i := 0; i < count; i++ {
		val, err := g.Registry.Generate(s.Items)
		if err != nil {
			return nil, err
		}
		arr[i] = val
	}

	if s.Unique {
		arr = deduplicate(arr)
	}

	return arr, nil
}

// --- ObjectGenerator ---

type ObjectGenerator struct {
	Registry *generator.Registry
}

func (g *ObjectGenerator) Name() string { return "object" }

func (g *ObjectGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeObject
}

func (g *ObjectGenerator) Generate(s *generator.Schema) (any, error) {
	obj := make(map[string]any)

	for name, propSchema := range s.Properties {
		val, err := g.Registry.Generate(propSchema)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		obj[name] = val
	}

	if s.AddlProps == nil || *s.AddlProps {
		for _, key := range s.Required {
			if _, exists := obj[key]; !exists {
				obj[key] = "required-" + key
			}
		}
	}

	return obj, nil
}

// --- FormatStringGenerator (catch-all for known formats) ---

type FormatStringGenerator struct{}

func (g *FormatStringGenerator) Name() string { return "format-string" }

func (g *FormatStringGenerator) CanHandle(s *generator.Schema) bool {
	return s.Type == generator.TypeString && s.Format != ""
}

func (g *FormatStringGenerator) Generate(s *generator.Schema) (any, error) {
	switch s.Format {
	case "ipv4":
		return randomIPv4(), nil
	case "ipv6":
		return randomIPv6(), nil
	case "hostname":
		return randomString(charsetAlpha, 8) + ".local", nil
	case "uri", "url":
		return "https://" + randomString(charsetAlnum, 8) + ".io/" + randomString(charsetAlnum, 4), nil
	case "byte":
		return randomString(charsetAlnum, 16), nil
	case "password":
		return randomString(charsetAlnum, 12), nil
	default:
		return randomString(charsetAlnum, 8), nil
	}
}

// --- helpers ---

func randomString(charset string, length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func randomIPv4() string {
	return strconv.Itoa(rand.Intn(223)+1) + "." +
		strconv.Itoa(rand.Intn(256)) + "." +
		strconv.Itoa(rand.Intn(256)) + "." +
		strconv.Itoa(rand.Intn(254)+1)
}

func randomIPv6() string {
	parts := make([]string, 8)
	for i := range parts {
		parts[i] = fmt.Sprintf("%x", rand.Intn(0x10000))
	}
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ":"
		}
		s += p
	}
	return s
}

func effectiveIntRange(s *generator.Schema) (int64, int64) {
	minVal := int64(0)
	maxVal := int64(100)

	if s.Minimum != nil {
		minVal = int64(*s.Minimum)
	}
	if s.ExclMin != nil {
		minVal = int64(*s.ExclMin) + 1
	}
	if s.Maximum != nil {
		maxVal = int64(*s.Maximum)
	}
	if s.ExclMax != nil {
		maxVal = int64(*s.ExclMax) - 1
	}

	return minVal, maxVal
}

func effectiveFloatRange(s *generator.Schema) (float64, float64) {
	minVal := 0.0
	maxVal := 100.0

	if s.Minimum != nil {
		minVal = *s.Minimum
	}
	if s.ExclMin != nil {
		minVal = *s.ExclMin + 0.001
	}
	if s.Maximum != nil {
		maxVal = *s.Maximum
	}
	if s.ExclMax != nil {
		maxVal = *s.ExclMax - 0.001
	}

	return minVal, maxVal
}

func deduplicate(arr []any) []any {
	seen := make(map[string]bool, len(arr))
	result := make([]any, 0, len(arr))
	for _, v := range arr {
		key := fmt.Sprintf("%v", v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result
}

// GeneratorInfo describes a single generator for the catalog API.
type GeneratorInfo struct {
	Name           string         `json:"name"`
	Label          string         `json:"label"`
	Description    string         `json:"description"`
	SchemaTemplate map[string]any `json:"schema_template"`
	Params         []ParamInfo    `json:"params,omitempty"`
}

// ParamInfo describes a configurable parameter for a generator.
type ParamInfo struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Default  any      `json:"default,omitempty"`
	Required bool     `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// CategoryInfo groups generators under a named category.
type CategoryInfo struct {
	Key        string          `json:"key"`
	Label      string          `json:"label"`
	Generators []GeneratorInfo `json:"generators"`
}

// Catalog returns the full generator catalog grouped by category.
func Catalog() []CategoryInfo {
	return []CategoryInfo{
		{
			Key:   "string",
			Label: "String",
			Generators: []GeneratorInfo{
				{
					Name:        "uuid",
					Label:       "UUID v4",
					Description: "Generate random UUID v4",
					SchemaTemplate: map[string]any{
						"type":   "string",
						"format": "uuid",
					},
				},
				{
					Name:        "email",
					Label:       "Email",
					Description: "Generate random email address",
					SchemaTemplate: map[string]any{
						"type":   "string",
						"format": "email",
					},
				},
				{
					Name:        "random-string",
					Label:       "Random String",
					Description: "Random alphanumeric string",
					SchemaTemplate: map[string]any{
						"type":      "string",
						"minLength": 8,
						"maxLength": 16,
					},
					Params: []ParamInfo{
						{Key: "minLength", Type: "integer", Default: 8},
						{Key: "maxLength", Type: "integer", Default: 16},
					},
				},
				{
					Name:        "enum-string",
					Label:       "Enum String",
					Description: "Pick from enum values",
					SchemaTemplate: map[string]any{
						"type": "string",
						"enum": []string{"option1", "option2"},
					},
					Params: []ParamInfo{
						{Key: "enum", Type: "array", Required: true},
					},
				},
				{
					Name:        "format-string",
					Label:       "Format String",
					Description: "Formatted string (ipv4/ipv6/url/date etc.)",
					SchemaTemplate: map[string]any{
						"type":   "string",
						"format": "ipv4",
					},
					Params: []ParamInfo{
						{Key: "format", Type: "string", Required: true, Enum: []string{"ipv4", "ipv6", "hostname", "uri", "url", "byte", "password"}},
					},
				},
			},
		},
		{
			Key:   "number",
			Label: "Number",
			Generators: []GeneratorInfo{
				{
					Name:        "random-int",
					Label:       "Random Integer",
					Description: "Random integer in range",
					SchemaTemplate: map[string]any{
						"type":    "integer",
						"minimum": float64(0),
						"maximum": float64(100),
					},
					Params: []ParamInfo{
						{Key: "minimum", Type: "integer", Default: 0},
						{Key: "maximum", Type: "integer", Default: 100},
					},
				},
				{
					Name:        "increment-int",
					Label:       "Increment Integer",
					Description: "Auto-incrementing integer",
					SchemaTemplate: map[string]any{
						"type":    "integer",
						"minimum": float64(0),
					},
					Params: []ParamInfo{
						{Key: "minimum", Type: "integer", Default: 0},
					},
				},
				{
					Name:        "random-float",
					Label:       "Random Float",
					Description: "Random float in range",
					SchemaTemplate: map[string]any{
						"type":       "number",
						"minimum":    float64(0),
						"maximum":    float64(100),
						"multipleOf": 0.01,
					},
					Params: []ParamInfo{
						{Key: "minimum", Type: "number", Default: float64(0)},
						{Key: "maximum", Type: "number", Default: float64(100)},
						{Key: "multipleOf", Type: "number", Default: 0.01},
					},
				},
			},
		},
		{
			Key:   "boolean",
			Label: "Boolean",
			Generators: []GeneratorInfo{
				{
					Name:        "random-bool",
					Label:       "Random Boolean",
					Description: "Random true/false",
					SchemaTemplate: map[string]any{
						"type": "boolean",
					},
				},
				{
					Name:        "weighted-bool",
					Label:       "Weighted Boolean",
					Description: "Boolean with weighted true ratio",
					SchemaTemplate: map[string]any{
						"type": "boolean",
					},
					Params: []ParamInfo{
						{Key: "trueWeight", Type: "number", Default: 0.5},
					},
				},
			},
		},
		{
			Key:   "composite",
			Label: "Composite",
			Generators: []GeneratorInfo{
				{
					Name:        "array",
					Label:       "Array",
					Description: "Array of generated items",
					SchemaTemplate: map[string]any{
						"type":     "array",
						"minItems": 1,
						"maxItems": 5,
					},
					Params: []ParamInfo{
						{Key: "minItems", Type: "integer", Default: 1},
						{Key: "maxItems", Type: "integer", Default: 5},
					},
				},
				{
					Name:        "object",
					Label:       "Object",
					Description: "Object with generated properties",
					SchemaTemplate: map[string]any{
						"type": "object",
					},
				},
			},
		},
	}
}

// DefaultRegistry returns a Registry with all built-in generators
// registered in the recommended order (specific → general).
func DefaultRegistry() *generator.Registry {
	r := generator.NewRegistry()

	r.Register(&UUIDGenerator{})
	r.Register(&EmailGenerator{})
	r.Register(&DateGenerator{})
	r.Register(&DateTimeGenerator{})
	r.Register(&FormatStringGenerator{})
	r.Register(&EnumString{})
	r.Register(&RandomString{})
	r.Register(&RandomInt{})
	r.Register(&RandomFloat{})
	r.Register(&RandomBool{})
	r.Register(&NullGenerator{})

	arrGen := &ArrayGenerator{Registry: r}
	r.Register(arrGen)

	objGen := &ObjectGenerator{Registry: r}
	r.Register(objGen)

	return r
}
