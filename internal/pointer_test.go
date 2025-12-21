package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPointer(t *testing.T) {
	// tests that a literal can be made a pointer
	val := 42
	ptr := Pointer(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)
}

func TestDereference(t *testing.T) {
	// returns value when not-nil
	val := 42
	ptr := Pointer(val)
	assert.Equal(t, val, Dereference(ptr))

	// returns zero value when nil
	ptr = nil
	assert.Equal(t, 0, Dereference(ptr))
}

func TestDereferenceArrays(t *testing.T) {
	// Check that arrays are dereferenced in a way we expect

	val := []string{"foo", "bar"}
	ptr := Pointer(val)
	assert.Equal(t, val, Dereference(ptr))

	ptr = nil
	assert.Equal(t, []string(nil), Dereference(ptr))
	assert.Equal(t, 0, len(Dereference(ptr))) // this is our most common check, so ensure it works
}
