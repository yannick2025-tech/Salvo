package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaType(t *testing.T) {
	assert.Equal(t, Type("string"), TypeString)
	assert.Equal(t, Type("number"), TypeNumber)
	assert.Equal(t, Type("integer"), TypeInteger)
	assert.Equal(t, Type("boolean"), TypeBoolean)
	assert.Equal(t, Type("array"), TypeArray)
	assert.Equal(t, Type("object"), TypeObject)
	assert.Equal(t, Type("null"), TypeNull)
}

func TestRegistryGenerateNil(t *testing.T) {
	r := NewRegistry()
	val, err := r.Generate(nil)
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestRegistryGenerateConst(t *testing.T) {
	r := NewRegistry()
	s := &Schema{HasConst: true, ConstVal: "hello"}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestRegistryGenerateDefault(t *testing.T) {
	r := NewRegistry()
	s := &Schema{HasDefault: true, DefaultVal: 42}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestRegistryGenerateEnum(t *testing.T) {
	r := NewRegistry()
	s := &Schema{Enum: []any{"a", "b", "c"}}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Contains(t, []any{"a", "b", "c"}, val)
}

func TestRegistryGenerateNoMatch(t *testing.T) {
	r := NewRegistry()
	s := &Schema{Type: TypeString}
	_, err := r.Generate(s)
	assert.Equal(t, ErrNoGenerator, err)
}

func TestRegistryRegisterAndGenerate(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubGenerator{typeName: "string"})

	s := &Schema{Type: TypeString}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Equal(t, "stub-value", val)
}

func TestRegistryFirstMatchWins(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubGenerator{typeName: "string", val: "first"})
	r.Register(&stubGenerator{typeName: "string", val: "second"})

	s := &Schema{Type: TypeString}
	val, err := r.Generate(s)
	require.NoError(t, err)
	assert.Equal(t, "first", val)
}

type stubGenerator struct {
	typeName string
	val      string
}

func (g *stubGenerator) Name() string { return "stub" }
func (g *stubGenerator) CanHandle(s *Schema) bool { return s.Type == Type(g.typeName) }
func (g *stubGenerator) Generate(_ *Schema) (any, error) {
	if g.val != "" {
		return g.val, nil
	}
	return "stub-value", nil
}

func TestErrors(t *testing.T) {
	assert.Error(t, ErrNoGenerator)
	assert.Error(t, ErrInvalidRange)
	assert.Error(t, ErrEmptyEnum)
}
