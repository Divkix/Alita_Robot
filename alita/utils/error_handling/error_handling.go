package error_handling

import (
	"fmt"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

// HandleErr handles errors by logging them.
func HandleErr(err error) {
	if err != nil {
		log.Error(err)
	}
}

// RecoverFromPanic recovers from a panic and logs it as an error.
// This should be used with defer in goroutines to prevent crashes.
// If Sentry is enabled, it will also capture the panic with full context.
func RecoverFromPanic(funcName, modName string) {
	if r := recover(); r != nil {
		err := fmt.Errorf("panic in %s.%s: %v", modName, funcName, r)
		stackTrace := string(debug.Stack())

		log.Errorf("[%s][%s] Recovered from panic: %v\nStack trace:\n%s",
			modName, funcName, r, stackTrace)

		// Send to Sentry if available
		if hub := sentry.CurrentHub().Clone(); hub.Client() != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetTag("module", modName)
				scope.SetTag("function", funcName)
				scope.SetLevel(sentry.LevelFatal)
				scope.SetContext("panic", map[string]interface{}{
					"recovered_value": fmt.Sprintf("%v", r),
					"stack_trace":     stackTrace,
				})
				hub.CaptureException(err)
			})
		}
	}
}

// CaptureError captures an error to Sentry with additional context.
// This is useful for capturing errors with custom tags and context
// without triggering a panic recovery flow.
func CaptureError(err error, tags map[string]string) {
	if err == nil {
		return
	}

	HandleErr(err) // Still log locally

	if hub := sentry.CurrentHub().Clone(); hub.Client() != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			for key, value := range tags {
				scope.SetTag(key, value)
			}
			hub.CaptureException(err)
		})
	}
}

// CaptureMessage sends a message to Sentry with optional tags.
// This is useful for tracking events that aren't errors but are worth monitoring.
func CaptureMessage(message string, tags map[string]string) {
	if hub := sentry.CurrentHub().Clone(); hub.Client() != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			for key, value := range tags {
				scope.SetTag(key, value)
			}
			hub.CaptureMessage(message)
		})
	}
}
