package filter

import (
	"context"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/media"
)

// Input - A filter input.
type Input struct {
	// The event to process/check.
	Event gomatrixserverlib.PDU

	// The confidence.Vectors so far. Note that the first set group will receive a classification.Spam vector of 0.5
	IncrementalConfidenceVectors confidence.Vectors

	// Extracted media items from the event.
	Medias []*media.Item

	// The context used for auditing the performance of policyserv's filters.
	auditContext *auditContext
}

// Instanced - A Set-specific filter.
type Instanced interface {
	// Name - The name of the filter for logging and metrics.
	Name() string

	// CheckEvent - Processes the given event, returning classifications about it. If an error occurred, the classifications
	// array will be nil/empty.
	CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error)
}

// CanBeInstanced - The base filter type, registered at compile/run time and used by Sets to create a long-lived
// Instanced instance.
type CanBeInstanced interface {
	// MakeFor - Creates a long-lived Instanced for the provided Set. If an error occurred, the Instanced will
	// be nil.
	MakeFor(set *Set) (Instanced, error)
}
