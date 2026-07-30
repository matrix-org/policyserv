package condition

// AnyIn - A shortcut to create an AnyOf condition for a set of values, like room IDs.
func AnyIn[T any](constructor func(val T) Condition, vals []T) Condition {
	conditions := make([]Condition, len(vals))
	for i, val := range vals {
		conditions[i] = constructor(val)
	}
	return AnyOf(conditions...)
}
