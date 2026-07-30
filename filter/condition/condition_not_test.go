package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNot(t *testing.T) {
	c := Not(AlwaysTrue(t))
	assert.False(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
}
