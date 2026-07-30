package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoomId(t *testing.T) {
	c := RoomId("!this one")
	assert.True(t, c.Matches(context.Background(), "whatever", "!this one", "whatever"))
	assert.False(t, c.Matches(context.Background(), "whatever", "not this one", "whatever"))
}
