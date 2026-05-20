// Package plugin defines the plugin system for Salvo. Plugins hook into
// the request execution lifecycle to add cross-cutting behaviour such
// as rate limiting, encryption, authentication, and logging.
//
// Design principles:
//   - Plugins are identified by a unique Name and an optional Priority.
//   - Lower priority values run first in the Before phase and last in
//     the After phase (onion model).
//   - Each plugin receives a Context that carries the request, response,
//     and arbitrary key-value pairs for inter-plugin communication.
package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

// Phase indicates which stage of the lifecycle the plugin hook is in.
type Phase int

const (
	PhaseBefore Phase = iota
	PhaseAfter
)

// Context carries data through the plugin pipeline. It wraps the
// standard context.Context and adds protocol-level request/response
// access plus a mutable key-value store for inter-plugin communication.
type Context struct {
	ctx      context.Context
	phase    Phase
	req      protocol.Request
	resp     protocol.Response
	store    map[string]any
	mu       sync.RWMutex
	aborted  bool
	abortErr error
}

// NewContext creates a plugin Context for the Before phase.
func NewContext(ctx context.Context, req protocol.Request) *Context {
	return &Context{
		ctx:   ctx,
		phase: PhaseBefore,
		req:   req,
		store: make(map[string]any),
	}
}

// Context returns the underlying context.Context.
func (c *Context) Context() context.Context {
	return c.ctx
}

// Phase returns the current lifecycle phase.
func (c *Context) Phase() Phase {
	return c.phase
}

// Request returns the protocol request.
func (c *Context) Request() protocol.Request {
	return c.req
}

// Response returns the protocol response (nil in Before phase).
func (c *Context) Response() protocol.Response {
	return c.resp
}

// SetResponse sets the response (used internally by the engine).
func (c *Context) SetResponse(resp protocol.Response) {
	c.resp = resp
}

// SetPhase sets the current phase (used internally by the engine).
func (c *Context) SetPhase(phase Phase) {
	c.phase = phase
}

// Set stores a key-value pair for inter-plugin communication.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	c.store[key] = value
	c.mu.Unlock()
}

// Get retrieves a value by key. Returns the value and true if found,
// or nil and false otherwise.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	v, ok := c.store[key]
	c.mu.RUnlock()
	return v, ok
}

// Abort stops the pipeline and returns the given error. Subsequent
// plugins in the current phase and all plugins in later phases are
// skipped.
func (c *Context) Abort(err error) {
	c.mu.Lock()
	c.aborted = true
	c.abortErr = err
	c.mu.Unlock()
}

// Aborted returns true if the pipeline was aborted.
func (c *Context) Aborted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aborted
}

// AbortError returns the error passed to Abort, or nil.
func (c *Context) AbortError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.abortErr
}

// Plugin is the interface that every plugin must implement.
type Plugin interface {
	// Name returns the unique plugin identifier.
	Name() string
	// Priority controls execution order. Lower values run first in
	// the Before phase and last in the After phase.
	Priority() int
	// Before is called before the request is executed. The plugin may
	// modify the request via the Context or abort the pipeline.
	Before(ctx *Context) error
	// After is called after the request is executed. The plugin may
	// inspect or modify the response via the Context.
	After(ctx *Context) error
}

// Registry manages a set of plugins and runs them in priority order.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	sorted  []Plugin
	dirty   bool
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry. Returns an error if a plugin
// with the same name is already registered.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("plugin: %q already registered", p.Name())
	}
	r.plugins[p.Name()] = p
	r.dirty = true
	return nil
}

// Unregister removes a plugin by name. Returns false if not found.
func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return false
	}
	delete(r.plugins, name)
	r.dirty = true
	return true
}

// Get returns a plugin by name, or nil if not found.
func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[name]
}

// List returns all plugins sorted by priority.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.rebuild()
	return append([]Plugin(nil), r.sorted...)
}

// RunBefore executes the Before hook of all plugins in priority order.
func (r *Registry) RunBefore(ctx *Context) error {
	for _, p := range r.List() {
		if ctx.Aborted() {
			return ctx.AbortError()
		}
		if err := p.Before(ctx); err != nil {
			return fmt.Errorf("plugin %q Before: %w", p.Name(), err)
		}
	}
	return nil
}

// RunAfter executes the After hook of all plugins in reverse priority
// order (onion model).
func (r *Registry) RunAfter(ctx *Context) error {
	plugins := r.List()
	for i := len(plugins) - 1; i >= 0; i-- {
		if ctx.Aborted() {
			return ctx.AbortError()
		}
		if err := plugins[i].After(ctx); err != nil {
			return fmt.Errorf("plugin %q After: %w", plugins[i].Name(), err)
		}
	}
	return nil
}

func (r *Registry) rebuild() {
	if !r.dirty {
		return
	}
	r.sorted = make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		r.sorted = append(r.sorted, p)
	}
	sort.Slice(r.sorted, func(i, j int) bool {
		return r.sorted[i].Priority() < r.sorted[j].Priority()
	})
	r.dirty = false
}
