package error_handling

import (
	"fmt"
	"runtime/debug"

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
func RecoverFromPanic(funcName, modName string) {
	if r := recover(); r != nil {
		stackTrace := string(debug.Stack())

		log.Errorf("[%s][%s] Recovered from panic: %v\nStack trace:\n%s",
			modName, funcName, r, stackTrace)
	}
}

// CaptureError logs an error with additional context tags.
// This is useful for capturing errors with custom tags for debugging.
func CaptureError(err error, tags map[string]string) {
	if err == nil {
		return
	}

	fields := log.Fields{}
	for key, value := range tags {
		fields[key] = value
	}
	fields["error"] = err.Error()

	log.WithFields(fields).Error(fmt.Sprintf("Error captured: %v", err))
}
