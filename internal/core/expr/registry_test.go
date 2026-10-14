// Package expr implements the expression engine for Salvo. It resolves
// ${...} expressions in strings, supporting variable references, function
// calls, and arithmetic operations.
package expr

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFunctionRegistry(t *testing.T) {
	r := NewFunctionRegistry()
	require.NotNil(t, r)
	assert.Empty(t, r.List())
}

func TestRegisterAndGet(t *testing.T) {
	r := NewFunctionRegistry()
	handler := func(args []string) (string, error) {
		return "ok", nil
	}

	err := r.Register("__test", handler)
	require.NoError(t, err)

	got, ok := r.Get("__test")
	require.True(t, ok)
	require.NotNil(t, got)

	result, err := got(nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewFunctionRegistry()
	handler := func(args []string) (string, error) { return "", nil }

	err := r.Register("__dup", handler)
	require.NoError(t, err)

	err = r.Register("__dup", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGetUnregistered(t *testing.T) {
	r := NewFunctionRegistry()
	_, ok := r.Get("__nonexistent")
	assert.False(t, ok)
}

func TestList(t *testing.T) {
	r := NewFunctionRegistry()
	handler := func(args []string) (string, error) { return "", nil }

	require.NoError(t, r.Register("__alpha", handler))
	require.NoError(t, r.Register("__beta", handler))
	require.NoError(t, r.Register("__gamma", handler))

	list := r.List()
	assert.Len(t, list, 3)

	// List should return sorted names.
	expected := []string{"__alpha", "__beta", "__gamma"}
	sort.Strings(expected)
	assert.Equal(t, expected, list)
}

func TestListEmpty(t *testing.T) {
	r := NewFunctionRegistry()
	list := r.List()
	assert.Empty(t, list)
}

func TestRegisterConcurrent(t *testing.T) {
	r := NewFunctionRegistry()
	handler := func(args []string) (string, error) { return "", nil }

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			_ = r.Register("__func"+string(rune('A'+idx%26))+string(rune('0'+idx/26)), handler)
			done <- true
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	// Should not panic; some registrations may fail due to duplicates
	// but the registry should be in a consistent state.
	_ = r.List()
}
