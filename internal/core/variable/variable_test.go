package variable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScope(t *testing.T) {
	s := NewScope()
	require.NotNil(t, s)
	assert.Equal(t, ScopeGlobal, s.Level())
}

func TestNewScopeWithLevel(t *testing.T) {
	s := NewScope(WithLevel(ScopeScene))
	assert.Equal(t, ScopeScene, s.Level())
}

func TestSetAndGet(t *testing.T) {
	s := NewScope()
	s.Set("key1", "value1")
	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestGetNonExistent(t *testing.T) {
	s := NewScope()
	_, ok := s.Get("missing")
	assert.False(t, ok)
}

func TestSetOverwrite(t *testing.T) {
	s := NewScope()
	s.Set("key", "old")
	s.Set("key", "new")
	val, ok := s.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "new", val)
}

func TestDelete(t *testing.T) {
	s := NewScope()
	s.Set("key", "value")
	s.Delete("key")
	_, ok := s.Get("key")
	assert.False(t, ok)
}

func TestKeys(t *testing.T) {
	s := NewScope()
	s.Set("a", 1)
	s.Set("b", 2)
	s.Set("c", 3)
	keys := s.Keys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")
	assert.Contains(t, keys, "c")
}

func TestClone(t *testing.T) {
	s := NewScope()
	s.Set("x", 10)
	s.Set("y", "hello")

	cloned := s.Clone()
	assert.Equal(t, s.Level(), cloned.Level())

	val, ok := cloned.Get("x")
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	// Modifying clone should not affect original.
	cloned.Set("x", 999)
	origVal, _ := s.Get("x")
	assert.Equal(t, 10, origVal)
}

func TestResolveGlobalOnly(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("host", "example.com")

	val, ok := Resolve(g, "host")
	assert.True(t, ok)
	assert.Equal(t, "example.com", val)
}

func TestResolveSceneOverridesGlobal(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("host", "global.com")
	g.Set("port", 8080)

	sc := NewScope(WithLevel(ScopeScene), WithParent(g))
	sc.Set("host", "scene.com")

	val, ok := Resolve(sc, "host")
	assert.True(t, ok)
	assert.Equal(t, "scene.com", val)

	val, ok = Resolve(sc, "port")
	assert.True(t, ok)
	assert.Equal(t, 8080, val)
}

func TestResolveAPIOverridesScene(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("timeout", 30)

	sc := NewScope(WithLevel(ScopeScene), WithParent(g))
	sc.Set("timeout", 10)

	api := NewScope(WithLevel(ScopeAPI), WithParent(sc))
	api.Set("timeout", 5)

	val, ok := Resolve(api, "timeout")
	assert.True(t, ok)
	assert.Equal(t, 5, val)
}

func TestResolveMissing(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	_, ok := Resolve(g, "nonexistent")
	assert.False(t, ok)
}

func TestResolveAll(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("host", "global.com")
	g.Set("port", 8080)

	sc := NewScope(WithLevel(ScopeScene), WithParent(g))
	sc.Set("host", "scene.com")
	sc.Set("path", "/api")

	api := NewScope(WithLevel(ScopeAPI), WithParent(sc))
	api.Set("path", "/api/v2")

	all := ResolveAll(api)
	assert.Equal(t, "scene.com", all["host"])
	assert.Equal(t, 8080, all["port"])
	assert.Equal(t, "/api/v2", all["path"])
}

func TestResolveString(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("name", "salvo")

	result, err := ResolveString(g, "Hello ${name}!")
	require.NoError(t, err)
	assert.Equal(t, "Hello salvo!", result)
}

func TestResolveStringMultipleVars(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("host", "example.com")
	g.Set("port", 8080)

	result, err := ResolveString(g, "http://${host}:${port}/api")
	require.NoError(t, err)
	assert.Equal(t, "http://example.com:8080/api", result)
}

func TestResolveStringMissingVar(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	_, err := ResolveString(g, "Hello ${missing}!")
	assert.Error(t, err)
}

func TestResolveStringNoVars(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	result, err := ResolveString(g, "plain text")
	require.NoError(t, err)
	assert.Equal(t, "plain text", result)
}

func TestResolveStringWithParent(t *testing.T) {
	g := NewScope(WithLevel(ScopeGlobal))
	g.Set("base", "global")

	sc := NewScope(WithLevel(ScopeScene), WithParent(g))
	sc.Set("suffix", "scene")

	result, err := ResolveString(sc, "${base}-${suffix}")
	require.NoError(t, err)
	assert.Equal(t, "global-scene", result)
}
