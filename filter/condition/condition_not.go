package condition

import "context"

// Not - Inverts the downstream condition's Matches call.
func Not(condition Condition) Condition {
	return newSimpleCondition(func(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
		return !condition.Matches(ctx, communityId, roomId, senderUserId)
	})
}
