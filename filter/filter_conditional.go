package filter

import (
	"context"
	"log"

	"github.com/matrix-org/policyserv/filter/condition"
	"github.com/matrix-org/policyserv/harms"
)

// ConditionalFilter - An instanced filter which only runs the downstream filter when the condition is met. Note
// that this implements all types of Instanced filters despite the downstream filter potentially not being so broad.
// This is done for ease of type definitions - a neutral content info will be returned if the filter is asked to
// check a content type that the downstream filter doesn't support.
//
// NOTE: Text filters don't have enough information to actually run the conditions, so InstancedTextFilter will always
// be run despite conditions. If a downstream filter is also an InstancedEventFilter, CheckEvent will be conditionally
// run.
type ConditionalFilter struct {
	set        *Set
	condition  condition.Condition
	downstream Instanced
}

func NewConditionalFilter(set *Set, downstream Instanced, condition condition.Condition) *ConditionalFilter {
	// Check early if the downstream is capable of being used with the ConditionalFilter.
	// Dev note: Update this check as ConditionalFilter gains more content types.
	_, isEventFilter := downstream.(InstancedEventFilter)
	_, isTextFilter := downstream.(InstancedTextFilter)
	if !isEventFilter && !isTextFilter {
		panic("developer error: expected downstream filter to be at least an event or text filter, got neither")
	}

	return &ConditionalFilter{
		set:        set,
		condition:  condition,
		downstream: downstream,
	}
}

// Name - Implements Instanced.
func (c *ConditionalFilter) Name() string {
	// Copy the name verbatim. We don't indicate that the filter is conditional in the name because that fact isn't
	// important for metrics or log line prefixes. We log elsewhere which conditions are being run.
	return c.downstream.Name()
}

// CheckEvent - Implements InstancedEventFilter.
func (c *ConditionalFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	eventFilter, ok := c.downstream.(InstancedEventFilter)
	if !ok {
		// Per docs on ConditionalFilter, return neutral when unsupported content type
		return harms.NeutralContent(), nil
	}

	if !c.condition.Matches(ctx, c.set.communityId, input.Event.RoomID().String(), string(input.Event.SenderID())) {
		log.Printf("[%s | %s] Condition did not match - returning neutral content to skip", input.Event.EventID(), input.Event.RoomID().String())
		return harms.NeutralContent(), nil
	}

	log.Printf("[%s | %s] Condition matched - calling downstream filter", input.Event.EventID(), input.Event.RoomID().String())
	return eventFilter.CheckEvent(ctx, input)
}

// CheckText - Implements InstancedTextFilter.
func (c *ConditionalFilter) CheckText(ctx context.Context, input string) (*harms.ContentInfo, error) {
	textFilter, ok := c.downstream.(InstancedTextFilter)
	if !ok {
		// Per docs on ConditionalFilter, return neutral when unsupported content type
		return harms.NeutralContent(), nil
	}
	// We don't have enough information to pass to the condition's Matches function, so run the filter unconditionally.
	return textFilter.CheckText(ctx, input)
}
