package storage

import "testing"

func TestNextId(t *testing.T) {
	t.Parallel()

	// We're not going to really ever be able to test uniqueness, but we can try to generate a bunch of IDs and see
	// how many collisions we come across.
	numIds := 100000
	seen := make(map[string]bool)
	for i := range numIds {
		id := NextId()
		if !seen[id] {
			seen[id] = true
		} else {
			t.Errorf("ID %s is repeated on loop %d", id, i)
		}
	}
}
