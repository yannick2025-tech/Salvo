// Package schema parses JSON Schema (Draft 7) and OpenAPI/Swagger
// specifications into the normalized generator.Schema representation.
package schema

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/yannick2025-tech/Salvo/internal/generator"
)

// Parse reads a JSON Schema from an io.Reader and returns a
// normalized generator.Schema.
func Parse(r io.Reader) (*generator.Schema, error) {
	var raw map[string]any
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("schema: decode: %w", err)
	}
	return parseObject(raw)
}

// ParseBytes parses a JSON Schema from a byte slice.
func ParseBytes(data []byte) (*generator.Schema, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("schema: unmarshal: %w", err)
	}
	return parseObject(raw)
}

// ParseOpenAPI parses an OpenAPI/Swagger specification and extracts
// the schemas from the components/schemas section.
func ParseOpenAPI(r io.Reader) (map[string]*generator.Schema, error) {
	var raw map[string]any
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("schema: decode openapi: %w", err)
	}

	schemas := make(map[string]*generator.Schema)

	components, _ := raw["components"].(map[string]any)
	if components == nil {
		defs, _ := raw["definitions"].(map[string]any)
		if defs != nil {
			components = map[string]any{"schemas": defs}
		}
	}

	schemaMap, _ := components["schemas"].(map[string]any)
	if schemaMap == nil {
		return schemas, nil
	}

	for name, def := range schemaMap {
		obj, ok := def.(map[string]any)
		if !ok {
			continue
		}
		s, err := parseObject(obj)
		if err != nil {
			return nil, fmt.Errorf("schema: parse %q: %w", name, err)
		}
		schemas[name] = s
	}

	return schemas, nil
}

func parseObject(raw map[string]any) (*generator.Schema, error) {
	s := &generator.Schema{}

	if v, ok := raw["type"]; ok {
		s.Type = generator.Type(fmt.Sprintf("%v", v))
	}

	s.Enum = parseAnySlice(raw["enum"])
	if v, ok := raw["const"]; ok {
		s.HasConst = true
		s.ConstVal = v
	}
	if v, ok := raw["default"]; ok {
		s.HasDefault = true
		s.DefaultVal = v
	}

	s.MinLength = parseIntPtr(raw["minLength"])
	s.MaxLength = parseIntPtr(raw["maxLength"])
	s.Pattern, _ = raw["pattern"].(string)
	s.Format, _ = raw["format"].(string)

	s.Minimum = parseFloatPtr(raw["minimum"])
	s.Maximum = parseFloatPtr(raw["maximum"])
	s.ExclMin = parseFloatPtr(raw["exclusiveMinimum"])
	s.ExclMax = parseFloatPtr(raw["exclusiveMaximum"])
	s.MultipleOf = parseFloatPtr(raw["multipleOf"])

	s.MinItems = parseIntPtr(raw["minItems"])
	s.MaxItems = parseIntPtr(raw["maxItems"])
	s.Unique = parseBool(raw["uniqueItems"])

	if v, ok := raw["items"].(map[string]any); ok {
		item, err := parseObject(v)
		if err != nil {
			return nil, err
		}
		s.Items = item
	}

	if v, ok := raw["properties"].(map[string]any); ok {
		s.Properties = make(map[string]*generator.Schema, len(v))
		for k, val := range v {
			obj, ok := val.(map[string]any)
			if !ok {
				continue
			}
			prop, err := parseObject(obj)
			if err != nil {
				return nil, fmt.Errorf("property %q: %w", k, err)
			}
			s.Properties[k] = prop
		}
	}

	s.Required = parseStringSlice(raw["required"])

	if v, ok := raw["additionalProperties"]; ok {
		if b, ok := v.(bool); ok {
			s.AddlProps = &b
		}
	}

	s.AllOf = parseSchemaSlice(raw["allOf"])
	s.AnyOf = parseSchemaSlice(raw["anyOf"])
	s.OneOf = parseSchemaSlice(raw["oneOf"])

	s.Title, _ = raw["title"].(string)
	s.Description, _ = raw["description"].(string)

	return s, nil
}

func parseAnySlice(v any) []any {
	if v == nil {
		return nil
	}
	s, ok := v.([]any)
	if !ok {
		return nil
	}
	return s
}

func parseIntPtr(v any) *int {
	if v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return nil
		}
		ii := int(i)
		return &ii
	}
	return nil
}

func parseFloatPtr(v any) *float64 {
	if v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		return &n
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func parseBool(v any) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func parseStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func parseSchemaSlice(v any) []*generator.Schema {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]*generator.Schema, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		s, err := parseObject(obj)
		if err != nil {
			continue
		}
		result = append(result, s)
	}
	return result
}
