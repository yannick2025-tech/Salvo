package expr

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// conditionExprRegex matches expressions like `${var} operator value`.
// Group 1: variable name, Group 2: operator, Group 3: value (optional, may be quoted).
var conditionExprRegex = regexp.MustCompile(`^\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}\s+(==|!=|>=|<=|>|<|equals|not_equals|greater_than_or_equal|gte|greater_than|less_than_or_equal|lte|less_than|empty|not_empty|size_equals|size_greater_than_or_equal|size_gte|size_greater_than|size_less_than_or_equal|size_lte|size_less_than)\s*(.*)$`)

// operatorMap maps symbolic operators to EvaluateCondition operator names.
var operatorMap = map[string]string{
	"==":       "equals",
	"!=":       "not_equals",
	">":        "greater_than",
	">=":       "greater_than_or_equal",
	"<":        "less_than",
	"<=":       "less_than_or_equal",
	"gte":      "greater_than_or_equal",
	"lte":      "less_than_or_equal",
	"size_gte": "size_greater_than_or_equal",
	"size_lte": "size_less_than_or_equal",
}

// EvaluateConditionExpr evaluates a condition expression string against the
// provided variables. Supports:
//
//	${var} operator value    — e.g. `${status} == "4"`, `${count} > 10`
//	${var}                   — truthy check (variable exists and not empty)
//
// If the expression doesn't match a known pattern, falls back to truthy
// evaluation (non-empty, not "false", not "0").
func EvaluateConditionExpr(expr string, variables map[string]any) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}

	// Try to parse as a structured condition expression.
	if variable, operator, value, ok := parseConditionExpr(expr); ok {
		return EvaluateCondition(variable, operator, value, variables)
	}

	// Fall back: bare `${var}` — truthy check.
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		varName := expr[2 : len(expr)-1]
		return EvaluateCondition(varName, "not_empty", "", variables)
	}

	// Fall back: negation `!${var}` — empty check.
	if strings.HasPrefix(expr, "!${") && strings.HasSuffix(expr, "}") {
		varName := expr[3 : len(expr)-1]
		return EvaluateCondition(varName, "empty", "", variables)
	}

	// Fall back: simple truthy evaluation.
	return expr != "" && expr != "false" && expr != "0" && expr != "''" && expr != "\"\""
}

// parseConditionExpr parses a condition expression like `${var} == "value"`.
// Returns the variable name, mapped operator name, value (unquoted), and true if parsed.
func parseConditionExpr(expr string) (variable, operator, value string, ok bool) {
	matches := conditionExprRegex.FindStringSubmatch(expr)
	if matches == nil {
		return "", "", "", false
	}

	variable = matches[1]
	opSym := matches[2]
	value = strings.TrimSpace(matches[3])

	// Map symbolic operator to canonical name.
	if mapped, exists := operatorMap[opSym]; exists {
		operator = mapped
	} else {
		operator = opSym
	}

	// Strip surrounding double quotes from value if present.
	value = strings.TrimPrefix(value, "\"")
	value = strings.TrimSuffix(value, "\"")

	return variable, operator, value, true
}

// EvaluateCondition evaluates a condition expression against the provided variables.
// It returns true if the condition is satisfied, false otherwise.
//
// Supported operators:
//   - equals / not_equals: string or numeric equality comparison
//   - greater_than / greater_than_or_equal / less_than / less_than_or_equal: numeric comparison
//   - empty / not_empty: checks variable existence and non-empty value
//   - size_equals / size_greater_than / size_greater_than_or_equal / size_less_than: array/slice length comparison
func EvaluateCondition(variable, operator, value string, variables map[string]any) bool {
	if variables == nil {
		// Only empty operator returns true for nil variables map.
		return operator == "empty"
	}

	raw, exists := variables[variable]
	if !exists {
		// Only empty operator returns true for non-existent variables.
		return operator == "empty"
	}

	switch operator {
	case "equals":
		return fmt.Sprintf("%v", raw) == value
	case "not_equals":
		return fmt.Sprintf("%v", raw) != value
	case "greater_than":
		cmp, ok := compareNumeric(raw, value)
		return ok && cmp > 0
	case "greater_than_or_equal":
		cmp, ok := compareNumeric(raw, value)
		return ok && cmp >= 0
	case "less_than":
		cmp, ok := compareNumeric(raw, value)
		return ok && cmp < 0
	case "less_than_or_equal":
		cmp, ok := compareNumeric(raw, value)
		return ok && cmp <= 0
	case "empty":
		return isEmpty(raw)
	case "not_empty":
		return !isEmpty(raw)
	case "size_equals":
		size := sliceLen(raw)
		if size < 0 {
			return false
		}
		expected, err := strconv.Atoi(value)
		if err != nil {
			return false
		}
		return size == expected
	case "size_greater_than":
		size := sliceLen(raw)
		if size < 0 {
			return false
		}
		expected, err := strconv.Atoi(value)
		if err != nil {
			return false
		}
		return size > expected
	case "size_greater_than_or_equal":
		size := sliceLen(raw)
		if size < 0 {
			return false
		}
		expected, err := strconv.Atoi(value)
		if err != nil {
			return false
		}
		return size >= expected
	case "size_less_than":
		size := sliceLen(raw)
		if size < 0 {
			return false
		}
		expected, err := strconv.Atoi(value)
		if err != nil {
			return false
		}
		return size < expected
	default:
		return false
	}
}

// compareNumeric compares a value against a string-encoded number.
// Returns: (1, true) if raw > value, (0, true) if equal, (-1, true) if raw < value.
// If either side is not numeric, returns (0, false) (not comparable).
func compareNumeric(raw any, value string) (int, bool) {
	a, ok := toFloat(raw)
	if !ok {
		return 0, false
	}
	b, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	if a > b {
		return 1, true
	}
	if a < b {
		return -1, true
	}
	return 0, true
}

// toFloat attempts to convert a value to float64.
func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// isEmpty checks if a value is considered empty (nil, zero value, or empty string).
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return !val
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return val == 0
	case float32, float64:
		return val == 0
	default:
		// For other types (slices, maps, etc.), check zero value.
		rv := reflect.ValueOf(v)
		return rv.IsZero()
	}
}

// sliceLen returns the length of a slice/array, or -1 if the value is not a slice/array.
func sliceLen(v any) int {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return rv.Len()
	default:
		return -1
	}
}
