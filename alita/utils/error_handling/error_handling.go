package error_handling

import (
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
		log.Errorf("[%s][%s] Recovered from panic: %v", modName, funcName, r)
	}
}
