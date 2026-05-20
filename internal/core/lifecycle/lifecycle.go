// Package lifecycle provides SETUP/TEARDOWN hook management for Salvo
// test scenarios.
//
// Hooks are organised at two levels:
//   - Global:  runs once before/after all scenarios.
//   - Scene:   runs once before/after each scenario.
//
// Multiple hooks can be registered for the same phase; they execute in
// registration order. If any hook returns an error, execution of the
// remaining hooks for that phase is stopped and the error is returned.
package lifecycle

import (
	"context"
	"fmt"
	"sync"
)

// Phase identifies a point in the test lifecycle where hooks run.
type Phase int

const (
	// HookGlobalSetup runs once before all scenarios.
	HookGlobalSetup Phase = iota
	// HookGlobalTeardown runs once after all scenarios.
	HookGlobalTeardown
	// HookSceneSetup runs before each scenario.
	HookSceneSetup
	// HookSceneTeardown runs after each scenario.
	HookSceneTeardown
)

// Hook is a function invoked at a specific lifecycle phase.
type Hook func(ctx context.Context) error

// Lifecycle manages the registration and execution of lifecycle hooks.
type Lifecycle struct {
	mu    sync.RWMutex
	hooks map[Phase][]Hook
}

// New creates a new Lifecycle with empty hook registries.
func New() *Lifecycle {
	return &Lifecycle{
		hooks: make(map[Phase][]Hook),
	}
}

// Register appends a hook for the given phase. Hooks execute in
// registration order.
func (l *Lifecycle) Register(phase Phase, hook Hook) {
	l.mu.Lock()
	l.hooks[phase] = append(l.hooks[phase], hook)
	l.mu.Unlock()
}

// Run executes all hooks registered for the given phase in order.
// If any hook returns an error, execution stops and the error is
// returned wrapped with the hook index.
func (l *Lifecycle) Run(ctx context.Context, phase Phase) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("lifecycle phase %d: %w", phase, err)
	}

	l.mu.RLock()
	hooks := make([]Hook, len(l.hooks[phase]))
	copy(hooks, l.hooks[phase])
	l.mu.RUnlock()

	for i, hook := range hooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("lifecycle hook %d in phase %d: %w", i, phase, err)
		}
	}
	return nil
}

// Clear removes all hooks for the given phase.
func (l *Lifecycle) Clear(phase Phase) {
	l.mu.Lock()
	delete(l.hooks, phase)
	l.mu.Unlock()
}

// ClearAll removes all hooks from all phases.
func (l *Lifecycle) ClearAll() {
	l.mu.Lock()
	l.hooks = make(map[Phase][]Hook)
	l.mu.Unlock()
}
