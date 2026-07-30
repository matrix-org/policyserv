package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyOf(t *testing.T) {
	c := AnyOf(
		AlwaysFalse(t),
		AlwaysFalse(t),
		AlwaysTrue(t),
	)
	assert.True(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))

	c = AnyOf(
		AlwaysFalse(t),
		AlwaysFalse(t),
		AlwaysTrue(t),
		FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
			t.Error("should not have been called")
			return false
		}),
	)
	assert.True(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
}
