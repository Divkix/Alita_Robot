package config

import (
	"strconv"
	"strings"
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
// it returns 0. This method ignores conversion errors for simplicity.
func (t typeConvertor) Int() int {
	val, _ := strconv.Atoi(t.str)
	return val
}

// Int64 converts the string value to a 64-bit integer. If the conversion fails,
// it returns 0. This method ignores conversion errors for simplicity.
func (t typeConvertor) Int64() int64 {
	val, _ := strconv.ParseInt(t.str, 10, 64)
	return val
}

// Bool converts the string value to a boolean. It returns true if the string
// equals "yes", "true", or "1" (case-insensitive), otherwise returns false.
func (t typeConvertor) Bool() bool {
	lower := strings.ToLower(strings.TrimSpace(t.str))
	return lower == "yes" || lower == "true" || lower == "1"
}

// Float64 converts the string value to a 64-bit float. If the conversion fails,
// it returns 0.0. This method ignores conversion errors for simplicity.
func (t typeConvertor) Float64() float64 {
	val, _ := strconv.ParseFloat(t.str, 64)
	return val
}
