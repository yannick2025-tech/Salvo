package so

import (
	"fmt"
	"strings"

	"github.com/yannick2025-tech/Salvo/internal/core/expr"
)

// RegisterSO registers the __so function into the given expression registry.
// The handler calls the named plugin's operation with the provided arguments.
//
// Expression syntax: ${__so("pluginName", "op", "arg1", "arg2", ...)}
// Versioned syntax:  ${__so("pluginName@1.0.0", "op", "arg1", ...)}
func RegisterSO(registry *expr.FunctionRegistry, loader *Loader) error {
	return registry.Register("__so", func(args []string) (string, error) {
		if len(args) < 2 {
			return "", fmt.Errorf("__so requires at least 2 arguments: name, op, [args...]")
		}

		nameVersion := args[0]
		op := args[1]
		callArgs := args[2:]

		// Parse name and optional version.
		pluginName, pluginVersion := parseNameVersion(nameVersion)

		p, ok := loader.Get(pluginName, pluginVersion)
		if !ok {
			return "", fmt.Errorf("__so: plugin %q not found", nameVersion)
		}

		result, err := p.Call(op, callArgs)
		if err != nil {
			return "", fmt.Errorf("__so: plugin %q op %q: %w", nameVersion, op, err)
		}

		return result, nil
	})
}

// parseNameVersion parses a plugin reference like "name@1.0.0" or just "name".
func parseNameVersion(input string) (name, version string) {
	parts := strings.SplitN(input, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return input, ""
}