package internal

// Pointer - utility function to convert a literal to a pointer. The `&` operator should be used for non-literal
// types, but this function will also work. See https://stackoverflow.com/a/30716481 for details.
func Pointer[T any](v T) *T {
	return &v
}

// Dereference - utility function to safely dereference a pointer to either its value or a zero value if nil.
func Dereference[T any](p *T) T {
	if p == nil {
		return *new(T)
	}
	return *p
}
