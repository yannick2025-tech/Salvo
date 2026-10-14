package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

func TestSafeGo_NormalExecution(t *testing.T) {
	l, _ := logger.New(logger.Config{Level: logger.DebugLevel})
	ctx := context.Background()

	var executed atomic.Bool
	safeGo(ctx, l, "test-normal", func() {
		executed.Store(true)
	})

	assert.Eventually(t, executed.Load, time.Second, 10*time.Millisecond,
		"goroutine should execute the function")
}

func TestSafeGo_PanicRecovery(t *testing.T) {
	l, _ := logger.New(logger.Config{Level: logger.DebugLevel})
	ctx := context.Background()

	called := make(chan struct{})
	safeGo(ctx, l, "test-panic", func() {
		close(called)
		panic("test panic value")
	})

	// Verify the function started executing (panic doesn't crash the process)
	select {
	case <-called:
		// Function was called before panic
	case <-time.After(time.Second):
		t.Fatal("goroutine did not execute within timeout")
	}

	// Allow goroutine to fully recover
	time.Sleep(50 * time.Millisecond)
}

func TestSafeGo_FunctionExecutesBeforePanic(t *testing.T) {
	l, _ := logger.New(logger.Config{Level: logger.DebugLevel})
	ctx := context.Background()

	var executed atomic.Bool
	safeGo(ctx, l, "test-exec-before-panic", func() {
		executed.Store(true)
		panic("panic after execution")
	})

	assert.Eventually(t, executed.Load, time.Second, 10*time.Millisecond,
		"function should execute before panic")
}

func TestSafeGo_NilContext(t *testing.T) {
	l, _ := logger.New(logger.Config{Level: logger.DebugLevel})

	var executed atomic.Bool
	safeGo(nil, l, "test-nil-ctx", func() {
		executed.Store(true)
	})

	assert.Eventually(t, executed.Load, time.Second, 10*time.Millisecond,
		"goroutine should execute with nil context")
}