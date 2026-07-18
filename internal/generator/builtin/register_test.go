package builtin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
)

func TestRegisterAll(t *testing.T) {
	r := expr.NewFunctionRegistry()
	RegisterAll(r)

	// All 6 functions should be registered
	expected := []string{
		"__email",
		"__manOf",
		"__oneOf",
		"__random",
		"__snowflakeId",
		"__weightedChoice",
	}
	registered := r.List()
	assert.Equal(t, expected, registered)

	// Each function should be callable
	t.Run("__random is callable", func(t *testing.T) {
		handler, ok := r.Get("__random")
		require.True(t, ok)
		res, err := handler([]string{"1", "100"})
		require.NoError(t, err)
		assert.NotEmpty(t, res)
	})

	t.Run("__snowflakeId is callable", func(t *testing.T) {
		handler, ok := r.Get("__snowflakeId")
		require.True(t, ok)
		res, err := handler(nil)
		require.NoError(t, err)
		assert.NotEmpty(t, res)
	})

	t.Run("__weightedChoice is callable", func(t *testing.T) {
		handler, ok := r.Get("__weightedChoice")
		require.True(t, ok)
		res, err := handler([]string{"A=50,B=50"})
		require.NoError(t, err)
		assert.Contains(t, []string{"A", "B"}, res)
	})

	t.Run("__oneOf is callable", func(t *testing.T) {
		handler, ok := r.Get("__oneOf")
		require.True(t, ok)
		res, err := handler([]string{"X", "Y", "Z"})
		require.NoError(t, err)
		assert.Contains(t, []string{"X", "Y", "Z"}, res)
	})

	t.Run("__manOf is callable", func(t *testing.T) {
		handler, ok := r.Get("__manOf")
		require.True(t, ok)
		res, err := handler([]string{"A", "B"})
		require.NoError(t, err)
		assert.NotEmpty(t, res)
	})

	t.Run("__email is callable", func(t *testing.T) {
		handler, ok := r.Get("__email")
		require.True(t, ok)
		res, err := handler(nil)
		require.NoError(t, err)
		assert.Contains(t, res, "@")
	})

	t.Run("__email with custom domain", func(t *testing.T) {
		handler, ok := r.Get("__email")
		require.True(t, ok)
		res, err := handler([]string{"gmail.com"})
		require.NoError(t, err)
		assert.Contains(t, res, "@gmail.com")
	})
}

func TestRegisterAllDuplicatePanics(t *testing.T) {
	r := expr.NewFunctionRegistry()
	RegisterAll(r)

	// Registering again should panic due to duplicate names
	assert.Panics(t, func() {
		RegisterAll(r)
	})
}

func TestWeightedChoiceAdapterNoArgs(t *testing.T) {
	r := expr.NewFunctionRegistry()
	RegisterAll(r)

	handler, ok := r.Get("__weightedChoice")
	require.True(t, ok)
	_, err := handler(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 1 argument")
}