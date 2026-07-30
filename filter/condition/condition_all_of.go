package condition

import "context"

// AllOf - ANDs all the conditions together, returning true if all the conditions return true. If a condition
// doesn't match, this won't check the remaining conditions.
func AllOf(conditions ...Condition) Condition {
	return newSimpleCondition(func(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
		for _, condition := range conditions {
			if !condition.Matches(ctx, communityId, roomId, senderUserId) {
				return false
			}
		}
		return true
	})
}
