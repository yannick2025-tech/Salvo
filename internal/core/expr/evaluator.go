package expr

import (
	"fmt"
	"reflect"
	"strconv"
)

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