package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/generator"
)

func TestParseStringSchema(t *testing.T) {
	input := `{"type": "string", "minLength": 5, "maxLength": 10}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeString, s.Type)
	require.NotNil(t, s.MinLength)
	assert.Equal(t, 5, *s.MinLength)
	require.NotNil(t, s.MaxLength)
	assert.Equal(t, 10, *s.MaxLength)
}

func TestParseIntegerSchema(t *testing.T) {
	input := `{"type": "integer", "minimum": 1, "maximum": 100}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeInteger, s.Type)
	require.NotNil(t, s.Minimum)
	assert.Equal(t, 1.0, *s.Minimum)
	require.NotNil(t, s.Maximum)
	assert.Equal(t, 100.0, *s.Maximum)
}

func TestParseNumberSchema(t *testing.T) {
	input := `{"type": "number", "exclusiveMinimum": 0, "exclusiveMaximum": 1}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeNumber, s.Type)
	require.NotNil(t, s.ExclMin)
	assert.Equal(t, 0.0, *s.ExclMin)
	require.NotNil(t, s.ExclMax)
	assert.Equal(t, 1.0, *s.ExclMax)
}

func TestParseStringWithFormat(t *testing.T) {
	input := `{"type": "string", "format": "email"}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeString, s.Type)
	assert.Equal(t, "email", s.Format)
}

func TestParseStringWithPattern(t *testing.T) {
	input := `{"type": "string", "pattern": "^[A-Z]{3}$"}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, "^[A-Z]{3}$", s.Pattern)
}

func TestParseEnum(t *testing.T) {
	input := `{"type": "string", "enum": ["red", "green", "blue"]}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, s.Enum, 3)
	assert.Contains(t, s.Enum, "red")
	assert.Contains(t, s.Enum, "green")
	assert.Contains(t, s.Enum, "blue")
}

func TestParseConst(t *testing.T) {
	input := `{"const": "fixed"}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.True(t, s.HasConst)
	assert.Equal(t, "fixed", s.ConstVal)
}

func TestParseDefault(t *testing.T) {
	input := `{"type": "integer", "default": 42}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.True(t, s.HasDefault)
	assert.Equal(t, 42.0, s.DefaultVal)
}

func TestParseArraySchema(t *testing.T) {
	input := `{
		"type": "array",
		"minItems": 1,
		"maxItems": 5,
		"uniqueItems": true,
		"items": {"type": "string"}
	}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeArray, s.Type)
	require.NotNil(t, s.MinItems)
	assert.Equal(t, 1, *s.MinItems)
	require.NotNil(t, s.MaxItems)
	assert.Equal(t, 5, *s.MaxItems)
	assert.True(t, s.Unique)
	require.NotNil(t, s.Items)
	assert.Equal(t, generator.TypeString, s.Items.Type)
}

func TestParseObjectSchema(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"],
		"additionalProperties": false
	}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, generator.TypeObject, s.Type)
	assert.Len(t, s.Properties, 2)
	assert.Contains(t, s.Properties, "name")
	assert.Contains(t, s.Properties, "age")
	assert.Equal(t, []string{"name"}, s.Required)
	require.NotNil(t, s.AddlProps)
	assert.False(t, *s.AddlProps)
}

func TestParseAllOf(t *testing.T) {
	input := `{
		"allOf": [
			{"type": "object", "properties": {"a": {"type": "string"}}},
			{"type": "object", "properties": {"b": {"type": "integer"}}}
		]
	}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, s.AllOf, 2)
}

func TestParseAnyOf(t *testing.T) {
	input := `{
		"anyOf": [
			{"type": "string"},
			{"type": "integer"}
		]
	}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, s.AnyOf, 2)
}

func TestParseOneOf(t *testing.T) {
	input := `{
		"oneOf": [
			{"type": "string"},
			{"type": "null"}
		]
	}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, s.OneOf, 2)
}

func TestParseMultipleOf(t *testing.T) {
	input := `{"type": "number", "multipleOf": 0.5}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, s.MultipleOf)
	assert.Equal(t, 0.5, *s.MultipleOf)
}

func TestParseBytes(t *testing.T) {
	data := []byte(`{"type": "boolean"}`)
	s, err := ParseBytes(data)
	require.NoError(t, err)
	assert.Equal(t, generator.TypeBoolean, s.Type)
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse(strings.NewReader("not json"))
	assert.Error(t, err)
}

func TestParseOpenAPI(t *testing.T) {
	input := `{
		"openapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0"},
		"paths": {},
		"components": {
			"schemas": {
				"User": {
					"type": "object",
					"properties": {
						"id": {"type": "string", "format": "uuid"},
						"name": {"type": "string"}
					}
				},
				"Order": {
					"type": "object",
					"properties": {
						"total": {"type": "number"}
					}
				}
			}
		}
	}`
	schemas, err := ParseOpenAPI(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, schemas, 2)

	user, ok := schemas["User"]
	require.True(t, ok)
	assert.Equal(t, generator.TypeObject, user.Type)
	assert.Len(t, user.Properties, 2)

	order, ok := schemas["Order"]
	require.True(t, ok)
	assert.Equal(t, generator.TypeObject, order.Type)
}

func TestParseOpenAPISwagger2(t *testing.T) {
	input := `{
		"swagger": "2.0",
		"info": {"title": "Test", "version": "1.0"},
		"paths": {},
		"definitions": {
			"Pet": {
				"type": "object",
				"properties": {
					"name": {"type": "string"}
				}
			}
		}
	}`
	schemas, err := ParseOpenAPI(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, schemas, 1)

	pet, ok := schemas["Pet"]
	require.True(t, ok)
	assert.Equal(t, generator.TypeObject, pet.Type)
}

func TestParseOpenAPINoComponents(t *testing.T) {
	input := `{"openapi": "3.0.0", "info": {"title": "T", "version": "1"}, "paths": {}}`
	schemas, err := ParseOpenAPI(strings.NewReader(input))
	require.NoError(t, err)
	assert.Empty(t, schemas)
}

func TestParseTitleAndDescription(t *testing.T) {
	input := `{"type": "string", "title": "Name", "description": "User name"}`
	s, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, "Name", s.Title)
	assert.Equal(t, "User name", s.Description)
}
