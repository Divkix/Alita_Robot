package helpers

// Ptr returns a pointer to the given value.
// Used for converting bool constants and other literals to pointer types
// required by gotgbot API structs (e.g., *bool fields).
func Ptr[T any](v T) *T {
	return &v
}
