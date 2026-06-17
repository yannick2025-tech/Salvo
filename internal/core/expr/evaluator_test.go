package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
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