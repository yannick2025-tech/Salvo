package expr

import (
	"fmt"
	"testing"
)

func TestMathExpressions(t *testing.T) {
	tests := []struct {
		input string
		vars  map[string]any
	}{
		{"${base} * 0.9", map[string]any{"base": "600"}},
		{"${base} * 0.25", map[string]any{"base": "600"}},
		{"${base} + 100", map[string]any{"base": "600"}},
		{"${base} / 2", map[string]any{"base": "600"}},
		{"${__random(100, 200)}", nil},
	}
	
	reg := NewFunctionRegistry()
	for _, tt := range tests {
		result, err := Resolve(tt.input, tt.vars, reg)
		fmt.Printf("Input: %q, Vars: %v\n  → Result: %q, Err: %v\n\n", tt.input, tt.vars, result, err)
	}
}
