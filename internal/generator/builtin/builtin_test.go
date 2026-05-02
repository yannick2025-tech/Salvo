package builtin

import (
	"fmt"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/generator"
)

func intPtr(v int) *int       { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestUUIDGenerator(t *testing.T) {
	g := &UUIDGenerator{}
	s := &generator.Schema{Type: generator.TypeString, Format: "uuid"}

	assert.True(t, g.CanHandle(s))
	assert.Equal(t, "uuid", g.Name())

	val, err := g.Generate(s)
	require.NoError(t, err)
	str, ok := val.(string)
	require.True(t, ok)
	assert.Len(t, str, 36)
	assert.Contains(t, str, "-")

	s2 := &generator.Schema{Type: generator.TypeString}
	assert.False(t, g.CanHandle(s2))
}

func TestUUIDGeneratorUniqueness(t *testing.T) {
	g := &UUIDGenerator{}
	s := &generator.Schema{Type: generator.TypeString, Format: "uuid"}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		str := val.(string)
		assert.False(t, seen[str], "duplicate UUID: %s", str)
		seen[str] = true
	}
}

func TestEmailGenerator(t *testing.T) {
	g := NewEmailGenerator()
	s := &generator.Schema{Type: generator.TypeString, Format: "email"}

	assert.True(t, g.CanHandle(s))
	assert.Equal(t, "email", g.Name())

	val, err := g.Generate(s)
	require.NoError(t, err)
	str, ok := val.(string)
	require.True(t, ok)
	_, err = mail.ParseAddress(str)
	assert.NoError(t, err)
}

func TestEmailGeneratorCustomDomains(t *testing.T) {
	g := NewEmailGenerator("corp.io")
	val, err := g.Generate(&generator.Schema{Type: generator.TypeString, Format: "email"})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(val.(string), "@corp.io"))
}

func TestDateGenerator(t *testing.T) {
	g := &DateGenerator{}
	s := &generator.Schema{Type: generator.TypeString, Format: "date"}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	str := val.(string)
	assert.Len(t, str, 10)
	assert.Contains(t, str, "-")
}

func TestDateTimeGenerator(t *testing.T) {
	g := &DateTimeGenerator{}
	s := &generator.Schema{Type: generator.TypeString, Format: "date-time"}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	str := val.(string)
	assert.Contains(t, str, "T")
}

func TestRandomString(t *testing.T) {
	g := &RandomString{}
	s := &generator.Schema{Type: generator.TypeString}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	str := val.(string)
	assert.GreaterOrEqual(t, len(str), 8)
	assert.LessOrEqual(t, len(str), 16)
}

func TestRandomStringWithLength(t *testing.T) {
	g := &RandomString{}
	s := &generator.Schema{
		Type:      generator.TypeString,
		MinLength: intPtr(5),
		MaxLength: intPtr(5),
	}
	val, err := g.Generate(s)
	require.NoError(t, err)
	assert.Len(t, val.(string), 5)
}

func TestRandomStringInvalidRange(t *testing.T) {
	g := &RandomString{}
	s := &generator.Schema{
		Type:      generator.TypeString,
		MinLength: intPtr(10),
		MaxLength: intPtr(5),
	}
	_, err := g.Generate(s)
	assert.Equal(t, generator.ErrInvalidRange, err)
}

func TestEnumString(t *testing.T) {
	g := &EnumString{}
	s := &generator.Schema{Enum: []any{"red", "green", "blue"}}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	assert.Contains(t, []any{"red", "green", "blue"}, val)
}

func TestEnumStringEmpty(t *testing.T) {
	g := &EnumString{}
	s := &generator.Schema{Enum: []any{}}
	_, err := g.Generate(s)
	assert.Equal(t, generator.ErrEmptyEnum, err)
}

func TestRandomInt(t *testing.T) {
	g := &RandomInt{}
	s := &generator.Schema{Type: generator.TypeInteger}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	n, ok := val.(int64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, n, int64(0))
	assert.LessOrEqual(t, n, int64(100))
}

func TestRandomIntWithRange(t *testing.T) {
	g := &RandomInt{}
	s := &generator.Schema{
		Type:    generator.TypeInteger,
		Minimum: floatPtr(10),
		Maximum: floatPtr(20),
	}
	for i := 0; i < 50; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		n := val.(int64)
		assert.GreaterOrEqual(t, n, int64(10))
		assert.LessOrEqual(t, n, int64(20))
	}
}

func TestRandomIntExclusiveRange(t *testing.T) {
	g := &RandomInt{}
	s := &generator.Schema{
		Type:    generator.TypeInteger,
		ExclMin: floatPtr(0),
		ExclMax: floatPtr(10),
	}
	for i := 0; i < 50; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		n := val.(int64)
		assert.GreaterOrEqual(t, n, int64(1))
		assert.LessOrEqual(t, n, int64(9))
	}
}

func TestIncrementInt(t *testing.T) {
	g := NewIncrementInt(100)
	s := &generator.Schema{Type: generator.TypeInteger}

	assert.Equal(t, "increment-int", g.Name())
	assert.True(t, g.CanHandle(s))

	for i := 0; i < 5; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		assert.Equal(t, int64(100+i), val.(int64))
	}
}

func TestRandomFloat(t *testing.T) {
	g := &RandomFloat{}
	s := &generator.Schema{Type: generator.TypeNumber}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	f, ok := val.(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, f, 0.0)
	assert.LessOrEqual(t, f, 100.0)
}

func TestRandomFloatWithRange(t *testing.T) {
	g := &RandomFloat{}
	s := &generator.Schema{
		Type:    generator.TypeNumber,
		Minimum: floatPtr(1.5),
		Maximum: floatPtr(3.5),
	}
	for i := 0; i < 50; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		f := val.(float64)
		assert.GreaterOrEqual(t, f, 1.5)
		assert.LessOrEqual(t, f, 3.5)
	}
}

func TestRandomFloatMultipleOf(t *testing.T) {
	g := &RandomFloat{}
	s := &generator.Schema{
		Type:       generator.TypeNumber,
		Minimum:    floatPtr(0),
		Maximum:    floatPtr(10),
		MultipleOf: floatPtr(0.5),
	}
	val, err := g.Generate(s)
	require.NoError(t, err)
	f := val.(float64)
	remainder := f - float64(int(f/0.5))*0.5
	assert.Less(t, remainder, 0.001)
}

func TestRandomBool(t *testing.T) {
	g := &RandomBool{}
	s := &generator.Schema{Type: generator.TypeBoolean}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	_, ok := val.(bool)
	assert.True(t, ok)
}

func TestWeightedBool(t *testing.T) {
	g := NewWeightedBool(0.9)
	s := &generator.Schema{Type: generator.TypeBoolean}

	assert.True(t, g.CanHandle(s))
	trueCount := 0
	for i := 0; i < 1000; i++ {
		val, err := g.Generate(s)
		require.NoError(t, err)
		if val.(bool) {
			trueCount++
		}
	}
	assert.Greater(t, trueCount, 700)
}

func TestWeightedBoolClamp(t *testing.T) {
	g := NewWeightedBool(-1)
	val, err := g.Generate(&generator.Schema{Type: generator.TypeBoolean})
	require.NoError(t, err)
	assert.IsType(t, true, val)

	g2 := NewWeightedBool(5)
	val2, err := g2.Generate(&generator.Schema{Type: generator.TypeBoolean})
	require.NoError(t, err)
	assert.IsType(t, true, val2)
}

func TestNullGenerator(t *testing.T) {
	g := &NullGenerator{}
	s := &generator.Schema{Type: generator.TypeNull}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestArrayGenerator(t *testing.T) {
	r := generator.NewRegistry()
	r.Register(&RandomString{})

	g := &ArrayGenerator{Registry: r}
	s := &generator.Schema{
		Type:     generator.TypeArray,
		MinItems: intPtr(3),
		MaxItems: intPtr(3),
		Items:    &generator.Schema{Type: generator.TypeString},
	}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	arr, ok := val.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 3)
}

func TestArrayGeneratorNoItems(t *testing.T) {
	r := generator.NewRegistry()
	g := &ArrayGenerator{Registry: r}
	s := &generator.Schema{
		Type:     generator.TypeArray,
		MinItems: intPtr(2),
		MaxItems: intPtr(2),
	}
	val, err := g.Generate(s)
	require.NoError(t, err)
	arr := val.([]any)
	assert.Len(t, arr, 2)
}

func TestArrayGeneratorInvalidRange(t *testing.T) {
	r := generator.NewRegistry()
	g := &ArrayGenerator{Registry: r}
	s := &generator.Schema{
		Type:     generator.TypeArray,
		MinItems: intPtr(10),
		MaxItems: intPtr(5),
	}
	_, err := g.Generate(s)
	assert.Equal(t, generator.ErrInvalidRange, err)
}

func TestArrayGeneratorUnique(t *testing.T) {
	r := generator.NewRegistry()
	r.Register(&RandomInt{})
	g := &ArrayGenerator{Registry: r}
	s := &generator.Schema{
		Type:     generator.TypeArray,
		MinItems: intPtr(20),
		MaxItems: intPtr(20),
		Unique:   true,
		Items:    &generator.Schema{Type: generator.TypeInteger, Minimum: floatPtr(0), Maximum: floatPtr(1000)},
	}
	val, err := g.Generate(s)
	require.NoError(t, err)
	arr := val.([]any)
	seen := make(map[string]bool)
	for _, v := range arr {
		key := fmt.Sprintf("%v", v)
		assert.False(t, seen[key], "duplicate: %v", v)
		seen[key] = true
	}
}

func TestObjectGenerator(t *testing.T) {
	r := generator.NewRegistry()
	r.Register(&RandomString{})
	r.Register(&RandomInt{})

	g := &ObjectGenerator{Registry: r}
	s := &generator.Schema{
		Type: generator.TypeObject,
		Properties: map[string]*generator.Schema{
			"name": {Type: generator.TypeString},
			"age":  {Type: generator.TypeInteger},
		},
		Required: []string{"name", "age"},
	}

	assert.True(t, g.CanHandle(s))
	val, err := g.Generate(s)
	require.NoError(t, err)
	obj, ok := val.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, obj, "name")
	assert.Contains(t, obj, "age")
}

func TestFormatStringGenerator(t *testing.T) {
	g := &FormatStringGenerator{}
	formats := []string{"ipv4", "ipv6", "hostname", "uri", "url", "byte", "password", "unknown"}

	for _, f := range formats {
		s := &generator.Schema{Type: generator.TypeString, Format: f}
		assert.True(t, g.CanHandle(s), "should handle format: %s", f)

		val, err := g.Generate(s)
		require.NoError(t, err, "format: %s", f)
		assert.NotEmpty(t, val.(string), "format: %s", f)
	}
}

func TestFormatStringIPv4(t *testing.T) {
	g := &FormatStringGenerator{}
	s := &generator.Schema{Type: generator.TypeString, Format: "ipv4"}
	val, err := g.Generate(s)
	require.NoError(t, err)
	str := val.(string)
	parts := strings.Split(str, ".")
	assert.Len(t, parts, 4)
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	assert.NotNil(t, r)

	s := &generator.Schema{Type: generator.TypeString, Format: "uuid"}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Len(t, val.(string), 36)
}

func TestDefaultRegistryFullObject(t *testing.T) {
	r := DefaultRegistry()
	s := &generator.Schema{
		Type: generator.TypeObject,
		Properties: map[string]*generator.Schema{
			"id":    {Type: generator.TypeString, Format: "uuid"},
			"name":  {Type: generator.TypeString},
			"age":   {Type: generator.TypeInteger, Minimum: floatPtr(18), Maximum: floatPtr(65)},
			"email": {Type: generator.TypeString, Format: "email"},
			"admin": {Type: generator.TypeBoolean},
		},
		Required: []string{"id", "name"},
	}

	val, err := r.Generate(s)
	require.NoError(t, err)
	obj := val.(map[string]any)
	assert.Contains(t, obj, "id")
	assert.Contains(t, obj, "name")
	assert.Contains(t, obj, "age")
	assert.Contains(t, obj, "email")
	assert.Contains(t, obj, "admin")

	id, ok := obj["id"].(string)
	require.True(t, ok)
	assert.Len(t, id, 36)
}
