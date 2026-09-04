package error_handling

import (
	"runtime/debug"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

var onErrorCallback atomic.Value

func SetOnErrorCallback(cb func()) {
	onErrorCallback.Store(cb)
}

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
