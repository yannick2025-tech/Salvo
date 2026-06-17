package builtin

import (
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/core/expr"
)

// RegisterAll registers all builtin system functions into the given FunctionRegistry.
func RegisterAll(r *expr.FunctionRegistry) {
	funcs := map[string]expr.FunctionHandler{
		"__weightedChoice": weightedChoiceAdapter,
		"__oneOf":          oneOfAdapter,
		"__manOf":          manOfAdapter,
		"__random":         Random, // Random already matches FunctionHandler signature
		"__snowflakeId":    snowflakeIdAdapter,
	}
	for name, handler := range funcs {
		// Registration should not fail on first call. Panic if it does.
		if err := r.Register(name, handler); err != nil {
			panic("builtin: register " + name + ": " + err.Error())
		}
	}
}

// weightedChoiceAdapter adapts WeightedChoice to FunctionHandler.
// The expression engine passes the weighted choice input as a single argument.
func weightedChoiceAdapter(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("__weightedChoice: requires at least 1 argument")
	}
	return WeightedChoice(args[0])
}

// oneOfAdapter adapts OneOf to FunctionHandler.
func oneOfAdapter(args []string) (string, error) {
	return OneOf(args)
}

// manOfAdapter adapts ManOf to FunctionHandler.
func manOfAdapter(args []string) (string, error) {
	return ManOf(args)
}

// snowflakeIdAdapter adapts SnowflakeId to FunctionHandler.
func snowflakeIdAdapter(_ []string) (string, error) {
	return SnowflakeId()
}