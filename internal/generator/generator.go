// Package generator provides parameter generation for Salvo test
// scenarios. It defines the Schema model (a normalized representation
// of JSON Schema Draft 7) and the Generator interface that all
// parameter generators must implement.
//
// Architecture:
//
//	JSON Schema / OpenAPI / Manual config
//	        │
//	        ▼
//	   Schema (normalized internal representation)
//	        │
//	        ▼
//	   Registry → picks matching Generator → Generate() → value
package generator

import (
	"fmt"
	"time"
)

// Type represents a JSON Schema type.
type Type string

const (
	TypeString  Type = "string"
	TypeNumber  Type = "number"
	TypeInteger Type = "integer"
	TypeBoolean Type = "boolean"
	TypeArray   Type = "array"
	TypeObject  Type = "object"
	TypeNull    Type = "null"
)

// Logger is the minimal interface required for function-level span logging.
// The generator package uses this interface to avoid importing the full logger.
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
}

// noopLogger silences log output when no logger is set.
type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

// Schema is the normalized internal representation of a JSON Schema.
// It captures all Draft 7 keywords that affect value generation.
type Schema struct {
	Type       Type  `json:"type"`
	Enum       []any `json:"enum,omitempty"`
	HasConst   bool  `json:"-"`
	ConstVal   any   `json:"const,omitempty"`
	HasDefault bool  `json:"-"`
	DefaultVal any   `json:"default,omitempty"`

	MinLength *int   `json:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Format    string `json:"format,omitempty"`

	Minimum    *float64 `json:"minimum,omitempty"`
	Maximum    *float64 `json:"maximum,omitempty"`
	ExclMin    *float64 `json:"exclusiveMinimum,omitempty"`
	ExclMax    *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf *float64 `json:"multipleOf,omitempty"`

	MinItems *int `json:"minItems,omitempty"`
	MaxItems *int `json:"maxItems,omitempty"`
	Unique   bool `json:"uniqueItems,omitempty"`

	Items      *Schema            `json:"items,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
	AddlProps  *bool              `json:"additionalProperties,omitempty"`

	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`

	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// Generator generates a value that conforms to a Schema.
type Generator interface {
	// Generate produces a value matching the given schema.
	Generate(schema *Schema) (any, error)
	// CanHandle returns true if this generator knows how to
	// produce values for the given schema.
	CanHandle(schema *Schema) bool
	// Name returns the generator identifier (e.g. "uuid").
	Name() string
}

// Registry holds all registered generators and dispatches schema
// generation to the first matching generator.
type Registry struct {
	generators []Generator
	log        Logger
}

// NewRegistry creates an empty generator registry.
func NewRegistry() *Registry {
	return &Registry{
		log: noopLogger{},
	}
}

// SetLogger attaches a logger for function-level span recording.
func (r *Registry) SetLogger(l Logger) {
	if l != nil {
		r.log = l
	}
}

// Register adds a generator to the registry. Generators registered
// earlier have higher priority (first-match wins).
func (r *Registry) Register(g Generator) {
	r.generators = append(r.generators, g)
}

// Generate walks the registered generators and delegates to the first
// one that can handle the schema. Returns an error if no generator
// matches.
func (r *Registry) Generate(schema *Schema) (any, error) {
	if schema == nil {
		return nil, nil
	}

	if schema.HasConst {
		r.log.Debug("generator: using const value",
			"generator", "const",
			"type", string(schema.Type),
		)
		return schema.ConstVal, nil
	}

	if schema.HasDefault {
		r.log.Debug("generator: using default value",
			"generator", "default",
			"type", string(schema.Type),
		)
		return schema.DefaultVal, nil
	}

	if len(schema.Enum) > 0 {
		r.log.Debug("generator: selecting from enum",
			"generator", "enum",
			"type", string(schema.Type),
			"enum_size", len(schema.Enum),
		)
		return schema.Enum[0], nil
	}

	for _, g := range r.generators {
		if g.CanHandle(schema) {
			start := time.Now()
			val, err := g.Generate(schema)
			elapsed := time.Since(start)
			if err != nil {
				r.log.Error("generator: function failed",
					"generator", g.Name(),
					"type", string(schema.Type),
					"elapsed_ms", elapsed.Milliseconds(),
					"error", err,
				)
				return nil, fmt.Errorf("generator %s: %w", g.Name(), err)
			}
			r.log.Debug("generator: function executed",
				"generator", g.Name(),
				"type", string(schema.Type),
				"elapsed_ms", elapsed.Milliseconds(),
			)
			return val, nil
		}
	}

	return nil, ErrNoGenerator
}
