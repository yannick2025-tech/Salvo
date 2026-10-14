package expr

import (
	"fmt"
	"regexp"
	"strings"
)

// funcPattern matches function calls like __funcName(args) or __funcName().
var funcPattern = regexp.MustCompile(`^__([a-zA-Z_][a-zA-Z0-9_]*)\((.*)\)$`)

// varPattern matches simple variable names (alphanumeric + underscore).
var varPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// maxResolveDepth limits recursive resolution to prevent infinite loops.
const maxResolveDepth = 10

// span represents a pair of start/end positions in a string.
type span struct {
	start, end int
}

// Resolve replaces all ${...} expressions in the input string with resolved
// values. It supports three expression types:
//  1. Variable reference: ${varName} — replaced by the variable's value
//  2. Function call: ${__funcName(args)} — calls a registered function
//  3. Math expression: mixed variables and operators — evaluated as arithmetic
func Resolve(input string, variables map[string]any, registry *FunctionRegistry) (string, error) {
	return resolveDepth(input, variables, registry, 0)
}

// resolveDepth performs recursive resolution with depth tracking.
func resolveDepth(input string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	if depth > maxResolveDepth {
		return "", fmt.Errorf("expression resolution exceeded max depth %d: possible circular reference", maxResolveDepth)
	}

	// Find top-level ${...} spans using brace counting.
	spans := findTopLevelBraces(input)
	if len(spans) == 0 {
		// No ${...} found, but still check if the entire input is a pure
		// arithmetic expression (e.g. "1100000001 / 100" after variables
		// have been resolved by the caller). If so, evaluate it as math.
		if isPureMathExpression(input) {
			result, err := EvalMath(input)
			if err == nil {
				return result, nil
			}
		}
		return input, nil
	}

	// Build result by resolving each span in order, left to right.
	var result strings.Builder
	lastEnd := 0

	for _, sp := range spans {
		// Append text between last resolved span and this one.
		result.WriteString(input[lastEnd:sp.start])

		// Extract and resolve the content between ${ and }.
		content := input[sp.start+2 : sp.end] // skip "${" and "}"

		resolved, err := resolveExpression(content, variables, registry, depth)
		if err != nil {
			return "", err
		}
		result.WriteString(resolved)
		lastEnd = sp.end + 1
	}

	// Append remaining text after last span.
	result.WriteString(input[lastEnd:])
	output := result.String()

	// After resolving all ${...}, check if the entire result is a pure
	// arithmetic expression. If so, evaluate it as math.
	if isPureMathExpression(output) {
		evalResult, mathErr := EvalMath(output)
		if mathErr == nil {
			return evalResult, nil
		}
	}

	return output, nil
}

// findTopLevelBraces finds all top-level ${...} spans in the input string.
// It uses brace counting to correctly handle nested ${} patterns.
// A separate braceDepth counter tracks non-${} braces (e.g. JSON object
// braces inside string arguments) so that } inside a JSON object does not
// close the outer ${...} prematurely.
func findTopLevelBraces(s string) []span {
	var spans []span
	n := len(s)
	i := 0

	for i < n {
		// Look for "${"
		idx := strings.Index(s[i:], "${")
		if idx == -1 {
			break
		}
		start := i + idx
		depth := 1
		braceDepth := 0
		j := start + 2 // skip "${"

		for j < n && depth > 0 {
			if s[j] == '{' {
				if j > 0 && s[j-1] == '$' {
					depth++
				} else {
					braceDepth++
				}
			} else if s[j] == '}' {
				if braceDepth > 0 {
					braceDepth--
				} else {
					depth--
				}
			}
			j++
		}

		if depth == 0 {
			// Found a matching pair.
			spans = append(spans, span{start: start, end: j - 1})
			i = j
		} else {
			// Unmatched opening brace, continue.
			i = start + 2
		}
	}

	return spans
}

// isPureMathExpression checks if a string contains only math characters.
func isPureMathExpression(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789.+-*/() \t\n\r", r) {
			return false
		}
	}
	return true
}

// resolveExpression resolves a single expression (content inside ${...}).
func resolveExpression(content string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}

	// First, resolve all nested ${...} within the content recursively.
	resolvedContent, err := resolveDepth(content, variables, registry, depth+1)
	if err != nil {
		return "", err
	}

	// Check if it's a function call (starts with __).
	if matches := funcPattern.FindStringSubmatch(resolvedContent); matches != nil {
		return resolveFunctionCall(matches[1], matches[2], variables, registry, depth)
	}

	// Check if it's a simple variable reference.
	if varPattern.MatchString(resolvedContent) {
		return resolveVariable(resolvedContent, variables, registry, depth)
	}

	// If the resolved content differs from original, it might now be evaluable.
	if resolvedContent != content {
		// Try math evaluation.
		if isPureMathExpression(resolvedContent) {
			result, err := EvalMath(resolvedContent)
			if err == nil {
				return result, nil
			}
		}
		return resolvedContent, nil
	}

	// Otherwise, treat as a math expression — resolve any bare variable names
	// within the content first, then evaluate.
	return resolveMathExpression(content, variables, registry, depth)
}

// resolveFunctionCall resolves a __funcName(args) expression.
func resolveFunctionCall(name, argsStr string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	// Resolve any nested ${...} expressions inside args first.
	resolvedArgs, err := resolveNestedInArgs(argsStr, variables, registry, depth)
	if err != nil {
		return "", fmt.Errorf("resolving args for %s: %w", name, err)
	}

	// Parse the resolved args string into individual arguments.
	args := parseArgs(resolvedArgs)

	// Look up the function (full name with __ prefix).
	fullName := "__" + name
	handler, ok := registry.Get(fullName)
	if !ok {
		// Unregistered function: preserve original text.
		return "${__" + name + "(" + argsStr + ")}", nil
	}

	result, err := handler(args)
	if err != nil {
		return "", fmt.Errorf("function %s: %w", name, err)
	}

	return result, nil
}

// resolveVariable resolves a variable name from the variables map.
// If the resolved value contains nested ${...}, it recursively resolves it.
func resolveVariable(name string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	if variables == nil {
		return "", fmt.Errorf("variable %q not found", name)
	}
	val, ok := variables[name]
	if !ok {
		return "", fmt.Errorf("variable %q not found", name)
	}
	s := fmt.Sprintf("%v", val)

	// Recursively resolve if the value contains nested expressions.
	if strings.Contains(s, "${") {
		resolved, err := resolveDepth(s, variables, registry, depth+1)
		if err != nil {
			return "", err
		}
		return resolved, nil
	}

	return s, nil
}

// resolveMathExpression resolves variables in a math expression and evaluates it.
func resolveMathExpression(content string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	// First resolve all nested ${...} in the content.
	resolved, err := resolveDepth(content, variables, registry, depth+1)
	if err != nil {
		return "", err
	}

	// Also replace any bare variable names (not in ${}) with their values.
	resolved = replaceBareVariables(resolved, variables)

	// Evaluate as arithmetic.
	result, err := EvalMath(resolved)
	if err != nil {
		// If evaluation fails, return the resolved string as-is.
		return resolved, nil
	}

	return result, nil
}

// replaceBareVariables replaces variable names in math expressions with their values.
func replaceBareVariables(input string, variables map[string]any) string {
	if variables == nil {
		return input
	}
	result := input
	for key, val := range variables {
		// Replace whole-word occurrences of variable names.
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(key) + `\b`)
		result = re.ReplaceAllString(result, fmt.Sprintf("%v", val))
	}
	return result
}

// resolveNestedInArgs resolves ${...} expressions inside function arguments.
func resolveNestedInArgs(argsStr string, variables map[string]any, registry *FunctionRegistry, depth int) (string, error) {
	if !strings.Contains(argsStr, "${") {
		return argsStr, nil
	}
	return resolveDepth(argsStr, variables, registry, depth+1)
}

// parseArgs parses a comma-separated argument string, respecting double-quoted
// and single-quoted strings.
// "a,b,c" → ["a", "b", "c"]
// `"a,b",c` → ["a,b", "c"]
// `'a,b',c` → ["a,b", "c"]
func parseArgs(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var args []string
	var current strings.Builder
	inDoubleQuote := false
	inSingleQuote := false

	for i := 0; i < len(input); i++ {
		c := input[i]
		switch {
		case c == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote
			current.WriteByte(c)
		case c == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote
			current.WriteByte(c)
		case c == ',' && !inDoubleQuote && !inSingleQuote:
			arg := strings.TrimSpace(current.String())
			arg = trimQuotes(arg)
			args = append(args, arg)
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		arg := strings.TrimSpace(current.String())
		arg = trimQuotes(arg)
		args = append(args, arg)
	}

	return args
}

// trimQuotes removes surrounding double or single quotes from a string if present.
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}
