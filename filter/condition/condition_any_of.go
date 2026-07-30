package condition

import "context"

// AnyOf - ORs each of the conditions, returning true when the first condition does. Does not check the
// remaining conditions.
func AnyOf(conditions ...Condition) Condition {
	return newSimpleCondition(func(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
		for _, condition := range conditions {
			if condition.Matches(ctx, communityId, roomId, senderUserId) {
				return true
			}
		}
		return false
	})
}
