package filter

import (
	"context"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/media"
)

// EventInput - An event with context to be provided to an InstancedEventFilter.
type EventInput struct {
	// The event to process/check.
	Event gomatrixserverlib.PDU

	// Extracted media items from the event.
	Medias []*media.Item

	// The context used for auditing the performance of policyserv's filters.
	auditContext *auditContext
}

// Instanced - A Set-specific filter.
type Instanced interface {
	// Name - The name of the filter for logging and metrics.
	Name() string
}

// CanBeInstanced - The base filter type, registered at compile/run time and used by Sets to create a long-lived
// Instanced instance.
type CanBeInstanced interface {
	// MakeFor - Creates a long-lived Instanced for the provided Set. If an error occurred, the Instanced will
	// be nil.
	MakeFor(set *Set) (Instanced, error)
}

// InstancedEventFilter - A filter which processes events.
type InstancedEventFilter interface {
	Instanced // parent type

	// CheckEvent - Processes the given content, returning harm/content classification. The content info may be nil
	// if there was an error.
	CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error)
}

type InstancedTextFilter interface {
	Instanced // parent type

	// CheckText - Processes the given text, returning harm/content classification. The content info may be nil if there
	// was an error. The input text string is assumed to be user-generated (message body, search query, etc) rather than
	// structured (JSON, CSV, etc).
	CheckText(ctx context.Context, input string) (*harms.ContentInfo, error)
}
