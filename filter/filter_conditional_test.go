package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/filter/condition"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestConditionalFilter(t *testing.T) {
	// Note: other tests ensure that the filter is properly created from the EnabledNames slice. This test
	// ensures that the condition is actually respected.

	event1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	event2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event2",
		RoomId:  "!wrong_room:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	set := &Set{
		// We don't use much of the Set in this case, so we can populate the minimum we need
		// rather than create it properly.

		communityId: "example",
	}
	instanced := &FixedInstancedFilter{
		T:   t,
		Set: set,
		Expect: &EventInput{
			Event:  event1,
			Medias: nil,
		},
		ExpectText: "testing123",
		ReturnInfo: harms.ProhibitedContent(harms.SpamFlooding), // so we can verify the filter was called
		ReturnErr:  nil,
	}
	cond := condition.RoomId("!foo:example.org")

	f := NewConditionalFilter(set, instanced, cond)
	assert.NotNil(t, f)

	// First event should go through to filter
	info, err := f.CheckEvent(context.Background(), &EventInput{
		Event:        event1,
		Medias:       nil,
		auditContext: &auditContext{}, // not used, but needs to be populated to appease FixedFilter
	})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.SpamFlooding), info)

	// Second event should not (wrong room ID)
	info, err = f.CheckEvent(context.Background(), &EventInput{
		Event:        event2,
		Medias:       nil,
		auditContext: &auditContext{}, // not used, but needs to be populated to appease FixedFilter
	})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), info)
}
