package utils

import (
	"log"
	"runtime/debug"
)

// SafeGo executes fn in a separate goroutine and recovers from any panic,
// logging the panic value and stack trace so the process does not terminate unexpectedly.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC_RECOVERED] Background goroutine panic: %v\nStack trace:\n%s", r, string(debug.Stack()))
			}
		}()
		fn()
	}()
}
