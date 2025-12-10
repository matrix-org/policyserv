package trust

import "context"

type Capability string

const CapabilityMedia Capability = "media"

// Source - represents a source of trust. "Trust" is arbitrarily defined as a set of capabilities applied to users
// in a room. This trust may be global, or it may be scoped to a community. Trust may also change over time.
type Source interface {
	// HasCapability returns TristateTrue if the given user has the given capability in the given room under this source of trust,
	// TristateFalse if they explicitly do not, and TristateDefault if the source of trust doesn't have an opinion.
	HasCapability(ctx context.Context, userId string, roomId string, capability Capability) (Tristate, error)
}
