package condition

import "context"

// Condition - Determines if a filter should be run based on the described condition.
type Condition interface {
	// Matches - Returns true if the filter should be run, false otherwise. Venue and sender information for the input
	// is provided, but notably the event/text/content itself is excluded. Filters check contents, conditions check venues.
	Matches(ctx context.Context, communityId string, roomId string, senderUserId string) bool
}

// Dev note: we could probably get away with Condition being a type alias for conditionFunc instead of being an interface
// type, but then we lose the ability to have more complex conditions with long-lived dependencies. So, we use the
// interface type early to avoid having to refactor later.
//
// It's also just clearer to see "condition.Matches(x)" than it is "condition(x)".

type conditionFunc func(ctx context.Context, communityId string, roomId string, senderUserId string) bool

// simpleCondition - Makes setting up conditions with few/no dependencies a bit easier.
type simpleCondition struct {
	fn conditionFunc
}

func newSimpleCondition(fn conditionFunc) Condition {
	return &simpleCondition{fn: fn}
}

func (c *simpleCondition) Matches(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
	return c.fn(ctx, communityId, roomId, senderUserId)
}
