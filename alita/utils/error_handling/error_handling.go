package error_handling

import (
	"runtime/debug"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

var onErrorCallback atomic.Value

// SetOnErrorCallback registers a callback to be called when an error is recovered from panic
func SetOnErrorCallback(cb func()) {
	onErrorCallback.Store(cb)
}

// RecoverFromPanic recovers from a panic and logs it as an error.
// This should be used with defer in goroutines to prevent crashes.
func RecoverFromPanic(funcName, modName string) {
	if r := recover(); r != nil {
		stackTrace := string(debug.Stack())

		log.Errorf("[%s][%s] Recovered from panic: %v\nStack trace:\n%s",
			modName, funcName, r, stackTrace)

		if cb, ok := onErrorCallback.Load().(func()); ok && cb != nil {
			cb()
		}
	}
}
