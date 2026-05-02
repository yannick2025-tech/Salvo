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
}

// NewRegistry creates an empty generator registry.
func NewRegistry() *Registry {
	return &Registry{}
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
		return schema.ConstVal, nil
	}

	if schema.HasDefault {
		return schema.DefaultVal, nil
	}

	if len(schema.Enum) > 0 {
		return schema.Enum[0], nil
	}

	for _, g := range r.generators {
		if g.CanHandle(schema) {
			return g.Generate(schema)
		}
	}

	return nil, ErrNoGenerator
}
