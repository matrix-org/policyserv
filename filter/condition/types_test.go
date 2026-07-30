package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestConditionMatchesFn func(communityId string, roomId string, senderUserId string) bool

func AlwaysTrue(t *testing.T) Condition {
	return FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
		return true
	})
}

func TestAlwaysTrue(t *testing.T) {
	c := AlwaysTrue(t)
	assert.True(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
}

func AlwaysFalse(t *testing.T) Condition {
	return FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
		return false
	})
}

func TestAlwaysFalse(t *testing.T) {
	c := AlwaysFalse(t)
	assert.False(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
}

type TestCondition struct {
	t  *testing.T
	fn TestConditionMatchesFn
}

func FuncCondition(t *testing.T, fn TestConditionMatchesFn) *TestCondition {
	return &TestCondition{
		t:  t,
		fn: fn,
	}
}

func (c *TestCondition) Matches(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
	assert.NotNil(c.t, ctx, "context is required")
	return c.fn(communityId, roomId, senderUserId)
}

func TestFuncCondition(t *testing.T) {
	c := FuncCondition(t, func(communityId string, roomId string, senderUserId string) bool {
		return communityId == "return true"
	})
	assert.True(t, c.Matches(context.Background(), "return true", "whatever", "whatever"))
	assert.False(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
}
