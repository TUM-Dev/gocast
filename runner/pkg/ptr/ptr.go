package ptr

// Take returns a pointer to the given value.
func Take[T any](t T) *T {
	return &t
}
