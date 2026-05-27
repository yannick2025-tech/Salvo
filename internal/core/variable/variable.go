// Package variable implements a three-level scoped variable system for
// Salvo test scenarios.
//
// Variables are organised in a hierarchy: Global → Scene → API. When a
// variable is looked up, the system walks from the current scope up to
// the root, returning the first match. This allows scene-level and
// API-level variables to override global defaults.
//
// String interpolation is supported via the ${var} syntax:
//
//	ResolveString(scope, "http://${host}:${port}/api")
package variable

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Level represents the scope depth of a variable store.
type Level int

const (
	// ScopeGlobal is the top-level scope shared across all scenarios.
	ScopeGlobal Level = iota
	// ScopeScene is the per-scenario scope that overrides global.
	ScopeScene
	// ScopeAPI is the per-API scope that overrides scene and global.
	ScopeAPI
)

// ScopeOption configures a Scope during construction.
type ScopeOption func(*Scope)

// WithLevel sets the scope level. Defaults to ScopeGlobal.
func WithLevel(level Level) ScopeOption {
	return func(s *Scope) {
		s.level = level
	}
}

// WithParent sets the parent scope for variable resolution chain.
func WithParent(parent *Scope) ScopeOption {
	return func(s *Scope) {
		s.parent = parent
	}
}

// Scope is a thread-safe key-value store with a hierarchical parent
// chain for variable resolution.
type Scope struct {
	mu     sync.RWMutex
	level  Level
	parent *Scope
	vars   map[string]any
}

// NewScope creates a new variable scope with the given options.
func NewScope(opts ...ScopeOption) *Scope {
	s := &Scope{
		vars: make(map[string]any),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Level returns the scope depth.
func (s *Scope) Level() Level {
	return s.level
}

// Set stores a key-value pair in this scope.
func (s *Scope) Set(key string, value any) {
	s.mu.Lock()
	s.vars[key] = value
	s.mu.Unlock()
}

// Get retrieves a value from this scope only (no parent lookup).
func (s *Scope) Get(key string) (any, bool) {
	s.mu.RLock()
	val, ok := s.vars[key]
	s.mu.RUnlock()
	return val, ok
}

// Delete removes a key from this scope.
func (s *Scope) Delete(key string) {
	s.mu.Lock()
	delete(s.vars, key)
	s.mu.Unlock()
}

// Keys returns all keys defined in this scope (excluding parent).
func (s *Scope) Keys() []string {
	s.mu.RLock()
	keys := make([]string, 0, len(s.vars))
	for k := range s.vars {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

// Clone creates a shallow copy of this scope. The parent reference is
// preserved but the variable map is independent.
func (s *Scope) Clone() *Scope {
	s.mu.RLock()
	cloned := &Scope{
		level:  s.level,
		parent: s.parent,
		vars:   make(map[string]any, len(s.vars)),
	}
	for k, v := range s.vars {
		cloned.vars[k] = v
	}
	s.mu.RUnlock()
	return cloned
}

// Resolve walks the scope chain from current to root and returns the
// first matching value.
func Resolve(scope *Scope, key string) (any, bool) {
	for cur := scope; cur != nil; cur = cur.parent {
		if val, ok := cur.Get(key); ok {
			return val, true
		}
	}
	return nil, false
}

// ResolveAll merges all variables from root to current scope, with
// child scopes overriding parent values.
func ResolveAll(scope *Scope) map[string]any {
	chain := make([]*Scope, 0)
	for cur := scope; cur != nil; cur = cur.parent {
		chain = append(chain, cur)
	}

	result := make(map[string]any)
	for i := len(chain) - 1; i >= 0; i-- {
		chain[i].mu.RLock()
		for k, v := range chain[i].vars {
			result[k] = v
		}
		chain[i].mu.RUnlock()
	}
	return result
}

// varPattern matches ${variable_name} placeholders.
var varPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// maxResolveDepth limits recursive variable resolution to prevent circular
// references from causing infinite recursion.
const maxResolveDepth = 10

// ResolveString replaces all ${var} placeholders in the input string
// with resolved variable values. If a resolved value itself contains
// ${var} references, they are resolved recursively up to maxResolveDepth
// levels. Returns an error if any referenced variable is not found or if
// the maximum resolution depth is exceeded (circular reference).
func ResolveString(scope *Scope, input string) (string, error) {
	return resolveStringDepth(scope, input, 0)
}

// resolveStringDepth performs recursive variable resolution with depth tracking.
func resolveStringDepth(scope *Scope, input string, depth int) (string, error) {
	if depth > maxResolveDepth {
		return "", fmt.Errorf("variable resolution exceeded max depth %d: possible circular reference", maxResolveDepth)
	}

	var err error
	result := varPattern.ReplaceAllStringFunc(input, func(match string) string {
		if err != nil {
			return match
		}
		key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		val, ok := Resolve(scope, key)
		if !ok {
			err = fmt.Errorf("variable %q not found in scope", key)
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if err != nil {
		return "", err
	}

	// If the result still contains ${var} patterns, resolve recursively.
	if varPattern.MatchString(result) {
		return resolveStringDepth(scope, result, depth+1)
	}

	return result, nil
}
