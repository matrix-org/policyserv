package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllOf(t *testing.T) {
	c := AllOf(
		AlwaysTrue(t),
		AlwaysFalse(t),
		FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
			t.Error("should not have been called")
			return false
		}),
	)
	assert.False(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))

	called := false
	c = AllOf(
		AlwaysTrue(t),
		AlwaysTrue(t),
		FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
			called = true
			return true
		}),
	)
	assert.True(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
	assert.True(t, called)
}
