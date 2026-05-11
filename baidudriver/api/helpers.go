package api

// Ptr returns a pointer to the given value.
// Useful for setting optional fields in request params.
func Ptr[T any](v T) *T {
	return &v
}
