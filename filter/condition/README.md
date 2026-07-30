# Conditions package - developer notes

Usually in a Go program the function which creates a `struct` would be something like `NewMyStruct`, but this package
does something a little different. To make the functions a bit more natural to read outside the package, the constructors
fit a more natural language instead.

For example:

```go
conditions.AllOf(
    conditions.TimeOfDay(x),
	conditions.AnyOf(
		conditions.RoomId(x),
		conditions.Not(
			conditions.AnyIn(conditions.CommunityId, []{x, y, z})
		),
	),
)
```

A normal Go program would look something like this:

```go
conditions.NewMatchAllCondition(
    conditions.NewTimeOfDayCondition(x),
	conditions.NewMatchAnyCondition(
		conditions.NewRoomIdCondition(x),
        conditions.NewInvertedCondition(
            conditions.NewMatchAnyInSliceCondition(conditions.CommunityId, []{x, y, z})
        ),
	),
)
```

... which isn't as nice as the first example.

To make the constructors easier to find, files in this package should fit the format `condition_{snake_case_constructor}.go`.
This means `AnyOf` becomes `condition_any_of.go`.