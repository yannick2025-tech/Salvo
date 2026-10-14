// Package expr implements the expression engine for Salvo. It resolves
// ${...} expressions in strings, supporting variable references, function
// calls, and arithmetic operations.
package expr

import (
	"sort"
	"sync"
)

// FunctionHandler is the signature for a registered system function.
// It receives parsed string arguments and returns a result or error.
type FunctionHandler func(args []string) (string, error)

// FunctionRegistry manages a thread-safe collection of named functions.
type FunctionRegistry struct {
	mu    sync.RWMutex
	funcs map[string]FunctionHandler
}

// NewFunctionRegistry creates an empty function registry.
func NewFunctionRegistry() *FunctionRegistry {
	return &FunctionRegistry{
		funcs: make(map[string]FunctionHandler),
	}
}

// Register adds a function handler under the given name. Returns an error
// if the name is already registered.
func (r *FunctionRegistry) Register(name string, handler FunctionHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.funcs[name]; exists {
		return &ErrDuplicateRegistration{Name: name}
	}
	r.funcs[name] = handler
	return nil
}

// Get retrieves a function handler by name. Returns false if not found.
func (r *FunctionRegistry) Get(name string) (FunctionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.funcs[name]
	return h, ok
}

// List returns all registered function names in sorted order.
func (r *FunctionRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
