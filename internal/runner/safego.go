package runner

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/yannick2025-tech/Salvo/internal/logger"
)

// safeGo spawns a goroutine that executes fn with panic recovery.
// If fn panics, the panic is recovered and logged at error level with
// the goroutine name, panic value, and full stack trace.
func safeGo(ctx context.Context, log logger.Logger, name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panicked",
					logger.F("goroutine", name),
					logger.F("panic", fmt.Sprintf("%v", r)),
					logger.F("stacktrace", string(debug.Stack())),
				)
			}
		}()
		fn()
	}()
}
