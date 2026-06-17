package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateCondition(t *testing.T) {
	t.Parallel()

	t.Run("equals", func(t *testing.T) {
		vars := map[string]any{"status": "4", "count": 5}
		assert.True(t, EvaluateCondition("status", "equals", "4", vars))
		assert.True(t, EvaluateCondition("count", "equals", "5", vars))
		assert.False(t, EvaluateCondition("status", "equals", "3", vars))
		assert.False(t, EvaluateCondition("count", "equals", "4", vars))
	})

	t.Run("not_equals", func(t *testing.T) {
		vars := map[string]any{"status": "COMPLETED"}
		assert.True(t, EvaluateCondition("status", "not_equals", "PENDING", vars))
		assert.False(t, EvaluateCondition("status", "not_equals", "COMPLETED", vars))
	})

	t.Run("greater_than", func(t *testing.T) {
		vars := map[string]any{"a": 5, "b": 0, "c": "abc"}
		assert.True(t, EvaluateCondition("a", "greater_than", "0", vars))
		assert.False(t, EvaluateCondition("b", "greater_than", "5", vars))
		assert.False(t, EvaluateCondition("a", "greater_than", "5", vars))
		// non-numeric value: not comparable, return false
		assert.False(t, EvaluateCondition("c", "greater_than", "0", vars))
	})

	t.Run("greater_than_or_equal", func(t *testing.T) {
		vars := map[string]any{"a": 5, "b": 4}
		assert.True(t, EvaluateCondition("a", "greater_than_or_equal", "5", vars))
		assert.True(t, EvaluateCondition("a", "greater_than_or_equal", "4", vars))
		assert.False(t, EvaluateCondition("b", "greater_than_or_equal", "5", vars))
	})

	t.Run("less_than", func(t *testing.T) {
		vars := map[string]any{"a": 3, "b": 10, "c": "abc"}
		assert.True(t, EvaluateCondition("a", "less_than", "10", vars))
		assert.False(t, EvaluateCondition("b", "less_than", "3", vars))
		assert.False(t, EvaluateCondition("a", "less_than", "3", vars))
		// non-numeric value: not comparable, return false
		assert.False(t, EvaluateCondition("c", "less_than", "10", vars))
	})

	t.Run("less_than_or_equal", func(t *testing.T) {
		vars := map[string]any{"a": 3, "b": 4}
		assert.True(t, EvaluateCondition("a", "less_than_or_equal", "3", vars))
		assert.True(t, EvaluateCondition("a", "less_than_or_equal", "4", vars))
		assert.False(t, EvaluateCondition("b", "less_than_or_equal", "3", vars))
	})

	t.Run("not_empty", func(t *testing.T) {
		vars := map[string]any{
			"has_value": "hello",
			"empty_str": "",
		}
		assert.True(t, EvaluateCondition("has_value", "not_empty", "", vars))
		assert.False(t, EvaluateCondition("empty_str", "not_empty", "", vars))
		assert.False(t, EvaluateCondition("nonexistent", "not_empty", "", vars))
	})

	t.Run("empty", func(t *testing.T) {
		vars := map[string]any{
			"has_value": "hello",
			"empty_str": "",
			"nil_val":   nil,
		}
		assert.False(t, EvaluateCondition("has_value", "empty", "", vars))
		assert.True(t, EvaluateCondition("empty_str", "empty", "", vars))
		assert.True(t, EvaluateCondition("nonexistent", "empty", "", vars))
		assert.True(t, EvaluateCondition("nil_val", "empty", "", vars))
	})

	t.Run("size_equals", func(t *testing.T) {
		vars := map[string]any{
			"items":     []string{"a", "b", "c"},
			"single":    []int{42},
			"empty_arr": []string{},
			"not_arr":   "string",
		}
		assert.True(t, EvaluateCondition("items", "size_equals", "3", vars))
		assert.False(t, EvaluateCondition("items", "size_equals", "2", vars))
		assert.False(t, EvaluateCondition("empty_arr", "size_equals", "1", vars))
		assert.False(t, EvaluateCondition("not_arr", "size_equals", "1", vars))
	})

	t.Run("size_greater_than", func(t *testing.T) {
		vars := map[string]any{
			"items":  []string{"a", "b", "c"},
			"single": []int{42},
		}
		assert.True(t, EvaluateCondition("items", "size_greater_than", "1", vars))
		assert.False(t, EvaluateCondition("single", "size_greater_than", "1", vars))
		assert.False(t, EvaluateCondition("items", "size_greater_than", "5", vars))
	})

	t.Run("size_greater_than_or_equal", func(t *testing.T) {
		vars := map[string]any{
			"items":  []string{"a", "b", "c"},
			"single": []int{42},
			"empty":  []string{},
		}
		assert.True(t, EvaluateCondition("items", "size_greater_than_or_equal", "3", vars))
		assert.True(t, EvaluateCondition("single", "size_greater_than_or_equal", "1", vars))
		assert.False(t, EvaluateCondition("empty", "size_greater_than_or_equal", "1", vars))
	})

	t.Run("size_less_than", func(t *testing.T) {
		vars := map[string]any{
			"items":  []string{"a", "b", "c"},
			"single": []int{42},
		}
		assert.True(t, EvaluateCondition("single", "size_less_than", "5", vars))
		assert.False(t, EvaluateCondition("items", "size_less_than", "3", vars))
		assert.False(t, EvaluateCondition("items", "size_less_than", "1", vars))
	})

	t.Run("variable not found", func(t *testing.T) {
		vars := map[string]any{"existing": "value"}
		// All operators except empty should return false for non-existent variables
		assert.False(t, EvaluateCondition("nonexistent", "equals", "x", vars))
		assert.False(t, EvaluateCondition("nonexistent", "not_equals", "x", vars))
		assert.False(t, EvaluateCondition("nonexistent", "greater_than", "0", vars))
		assert.False(t, EvaluateCondition("nonexistent", "size_equals", "1", vars))
		// empty is the only operator that returns true for non-existent variables
		assert.True(t, EvaluateCondition("nonexistent", "empty", "", vars))
		assert.False(t, EvaluateCondition("nonexistent", "not_empty", "", vars))
	})

	t.Run("unknown operator returns false", func(t *testing.T) {
		vars := map[string]any{"x": "value"}
		assert.False(t, EvaluateCondition("x", "unknown_op", "", vars))
	})

	t.Run("nil variables map", func(t *testing.T) {
		assert.False(t, EvaluateCondition("x", "equals", "y", nil))
		assert.True(t, EvaluateCondition("x", "empty", "", nil))
	})

	t.Run("non-numeric comparison returns false", func(t *testing.T) {
		vars := map[string]any{"x": "abc"}
		assert.False(t, EvaluateCondition("x", "greater_than", "5", vars))
		assert.False(t, EvaluateCondition("x", "less_than", "5", vars))
	})

	t.Run("value empty string comparison equals", func(t *testing.T) {
		vars := map[string]any{"x": ""}
		assert.True(t, EvaluateCondition("x", "equals", "", vars))
	})

	t.Run("int and float cross-type comparison", func(t *testing.T) {
		vars := map[string]any{"intVal": 5, "floatVal": 5.0}
		assert.True(t, EvaluateCondition("intVal", "greater_than", "4", vars))
		assert.True(t, EvaluateCondition("floatVal", "equals", "5", vars))
	})

	t.Run("non-array variable for size_equals returns false", func(t *testing.T) {
		vars := map[string]any{"x": "not an array"}
		assert.False(t, EvaluateCondition("x", "size_equals", "1", vars))
	})
}

func TestEvaluateConditionConcurrent(t *testing.T) {
	runs := 100
	done := make(chan bool, runs)
	vars := map[string]any{
		"status": "4",
		"count":  10,
		"items":  []string{"a", "b", "c"},
	}
	for i := 0; i < runs; i++ {
		go func() {
			assert.True(t, EvaluateCondition("status", "equals", "4", vars))
			assert.True(t, EvaluateCondition("count", "greater_than", "5", vars))
			assert.True(t, EvaluateCondition("items", "size_equals", "3", vars))
			done <- true
		}()
	}
	for i := 0; i < runs; i++ {
		<-done
	}
}

func TestEvaluateConditionExpr(t *testing.T) {
	t.Parallel()

	t.Run("empty expr returns true", func(t *testing.T) {
		assert.True(t, EvaluateConditionExpr("", nil))
	})

	t.Run("${var} == value with quotes", func(t *testing.T) {
		vars := map[string]any{"status": "4"}
		assert.True(t, EvaluateConditionExpr(`${status} == "4"`, vars))
		assert.False(t, EvaluateConditionExpr(`${status} == "3"`, vars))
	})

	t.Run("${var} == value without quotes", func(t *testing.T) {
		vars := map[string]any{"status": "4"}
		assert.True(t, EvaluateConditionExpr(`${status} == 4`, vars))
	})

	t.Run("${var} != value", func(t *testing.T) {
		vars := map[string]any{"status": "COMPLETED"}
		assert.True(t, EvaluateConditionExpr(`${status} != "PENDING"`, vars))
		assert.False(t, EvaluateConditionExpr(`${status} != "COMPLETED"`, vars))
	})

	t.Run("${var} > numeric", func(t *testing.T) {
		vars := map[string]any{"count": 10}
		assert.True(t, EvaluateConditionExpr(`${count} > 5`, vars))
		assert.False(t, EvaluateConditionExpr(`${count} > 10`, vars))
		assert.False(t, EvaluateConditionExpr(`${count} > 20`, vars))
	})

	t.Run("${var} >= numeric", func(t *testing.T) {
		vars := map[string]any{"count": 10}
		assert.True(t, EvaluateConditionExpr(`${count} >= 10`, vars))
		assert.True(t, EvaluateConditionExpr(`${count} >= 5`, vars))
		assert.False(t, EvaluateConditionExpr(`${count} >= 20`, vars))
	})

	t.Run("${var} < numeric", func(t *testing.T) {
		vars := map[string]any{"count": 10}
		assert.True(t, EvaluateConditionExpr(`${count} < 20`, vars))
		assert.False(t, EvaluateConditionExpr(`${count} < 10`, vars))
	})

	t.Run("${var} <= numeric", func(t *testing.T) {
		vars := map[string]any{"count": 10}
		assert.True(t, EvaluateConditionExpr(`${count} <= 10`, vars))
		assert.False(t, EvaluateConditionExpr(`${count} <= 5`, vars))
	})

	t.Run("${var} equals keyword", func(t *testing.T) {
		vars := map[string]any{"status": "4"}
		assert.True(t, EvaluateConditionExpr(`${status} equals "4"`, vars))
	})

	t.Run("${var} not_equals keyword", func(t *testing.T) {
		vars := map[string]any{"status": "4"}
		assert.False(t, EvaluateConditionExpr(`${status} not_equals "4"`, vars))
	})

	t.Run("${var} greater_than keyword", func(t *testing.T) {
		vars := map[string]any{"count": 10}
		assert.True(t, EvaluateConditionExpr(`${count} greater_than 5`, vars))
	})

	t.Run("bare ${var} truthy check", func(t *testing.T) {
		assert.True(t, EvaluateConditionExpr(`${existing}`, map[string]any{"existing": "value"}))
		assert.False(t, EvaluateConditionExpr(`${missing}`, map[string]any{}))
		assert.False(t, EvaluateConditionExpr(`${empty}`, map[string]any{"empty": ""}))
	})

	t.Run("!${var} negation check", func(t *testing.T) {
		assert.True(t, EvaluateConditionExpr(`!${missing}`, map[string]any{}))
		assert.False(t, EvaluateConditionExpr(`!${existing}`, map[string]any{"existing": "value"}))
	})

	t.Run("size_equals expr", func(t *testing.T) {
		vars := map[string]any{"items": []string{"a", "b", "c"}}
		assert.True(t, EvaluateConditionExpr(`${items} size_equals 3`, vars))
		assert.False(t, EvaluateConditionExpr(`${items} size_equals 2`, vars))
	})

	t.Run("fallback truthy for non-matching exprs", func(t *testing.T) {
		assert.True(t, EvaluateConditionExpr("true", nil))
		assert.True(t, EvaluateConditionExpr("some_text", nil))
		assert.False(t, EvaluateConditionExpr("false", nil))
		assert.False(t, EvaluateConditionExpr("0", nil))
	})

	t.Run("size_less_than with keyword", func(t *testing.T) {
		vars := map[string]any{"items": []string{"a"}}
		assert.True(t, EvaluateConditionExpr(`${items} size_less_than 5`, vars))
		assert.False(t, EvaluateConditionExpr(`${items} size_less_than 1`, vars))
	})

	t.Run("concurrent safety", func(t *testing.T) {
		vars := map[string]any{"status": "4", "count": 10}
		done := make(chan bool, 100)
		for i := 0; i < 100; i++ {
			go func() {
				assert.True(t, EvaluateConditionExpr(`${status} == "4"`, vars))
				assert.True(t, EvaluateConditionExpr(`${count} > 5`, vars))
				done <- true
			}()
		}
		for i := 0; i < 100; i++ {
			<-done
		}
	})
}

func TestParseConditionExpr(t *testing.T) {
	t.Parallel()

	t.Run("variable with quoted value", func(t *testing.T) {
		v, op, val, ok := parseConditionExpr(`${status} == "4"`)
		require.True(t, ok)
		assert.Equal(t, "status", v)
		assert.Equal(t, "equals", op)
		assert.Equal(t, "4", val)
	})

	t.Run("variable with unquoted numeric", func(t *testing.T) {
		v, op, val, ok := parseConditionExpr(`${count} > 10`)
		require.True(t, ok)
		assert.Equal(t, "count", v)
		assert.Equal(t, "greater_than", op)
		assert.Equal(t, "10", val)
	})

	t.Run("variable with keyword operator", func(t *testing.T) {
		v, op, val, ok := parseConditionExpr(`${status} equals "COMPLETED"`)
		require.True(t, ok)
		assert.Equal(t, "status", v)
		assert.Equal(t, "equals", op)
		assert.Equal(t, "COMPLETED", val)
	})

	t.Run("not matching returns false", func(t *testing.T) {
		_, _, _, ok := parseConditionExpr("just some text")
		assert.False(t, ok)
	})

	t.Run("empty expr returns false", func(t *testing.T) {
		_, _, _, ok := parseConditionExpr("")
		assert.False(t, ok)
	})

	t.Run("not_empty keyword no value", func(t *testing.T) {
		v, op, val, ok := parseConditionExpr(`${var} not_empty`)
		require.True(t, ok)
		assert.Equal(t, "var", v)
		assert.Equal(t, "not_empty", op)
		assert.Equal(t, "", val)
	})
}