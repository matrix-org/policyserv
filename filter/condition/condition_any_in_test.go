package condition

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyIn(t *testing.T) {
	called := make([]bool, 3)

	funcConditionProxy := func(fn TestConditionMatchesFn) Condition {
		return FuncCondition(t, fn)
	}

	vals := []TestConditionMatchesFn{
		func(communityId string, roomId string, senderUserId string) bool {
			called[0] = true
			return false
		},
		func(communityId string, roomId string, senderUserId string) bool {
			called[1] = true
			return false
		},
		func(communityId string, roomId string, senderUserId string) bool {
			called[2] = true
			return true
		},
	}
	c := AnyIn(funcConditionProxy, vals)
	assert.True(t, c.Matches(context.Background(), "whatever", "whatever", "whatever"))
	assert.True(t, called[0])
	assert.True(t, called[1])
	assert.True(t, called[2])
}
