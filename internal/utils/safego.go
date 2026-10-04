package utils

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// SafeGo executes fn in a separate goroutine and recovers from any panic,
// logging the panic value and stack trace so the process does not terminate unexpectedly.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic_recovered_in_goroutine",
					"panic", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}
