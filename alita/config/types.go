package config

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// typeConvertor is a struct that will convert a string to a specific type
type typeConvertor struct {
	str string
}

// StringArray converts a comma-separated string into a slice of trimmed strings.
// It splits the input string by commas and removes leading/trailing whitespace
// from each element.
func (t typeConvertor) StringArray() []string {
	allUpdates := strings.Split(t.str, ",")
	for i, j := range allUpdates {
		allUpdates[i] = strings.TrimSpace(j) // this will trim the whitespace
	}
	return allUpdates
}

// Int converts the string value to an integer. If the conversion fails,
// it logs a warning and returns 0. Empty strings return 0 silently (expected for optional config).
func (t typeConvertor) Int() int {
	if t.str == "" {
		return 0
	}
	val, err := strconv.Atoi(t.str)
	if err != nil {
		log.WithError(err).WithField("value", t.str).Warn("Failed to convert config value to int")
		return 0
	}
	return val
}

// Int64 converts the string value to a 64-bit integer. If the conversion fails,
// it logs a warning and returns 0. Empty strings return 0 silently (expected for optional config).
func (t typeConvertor) Int64() int64 {
	if t.str == "" {
		return 0
	}
	val, err := strconv.ParseInt(t.str, 10, 64)
	if err != nil {
		log.WithError(err).WithField("value", t.str).Warn("Failed to convert config value to int64")
		return 0
	}
	return val
}

// Bool converts the string value to a boolean. It returns true if the string
// equals "yes", "true", or "1" (case-insensitive), otherwise returns false.
func (t typeConvertor) Bool() bool {
	lower := strings.ToLower(strings.TrimSpace(t.str))
	return lower == "yes" || lower == "true" || lower == "1"
}
