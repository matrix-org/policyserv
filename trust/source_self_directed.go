package trust

import (
	"context"
	"log"

	"github.com/matrix-org/policyserv/storage"
	"github.com/ryanuber/go-glob"
)

// SelfDirectedSource - trusts user IDs matching the allowed globs list, and doesn't trust those matching the
// denied globs list. Note that the denied globs list takes precedence over the allowed globs list.
type SelfDirectedSource struct {
	db           storage.PersistentStorage
	allowedGlobs []string
	deniedGlobs  []string
}

func NewSelfDirectedSource(db storage.PersistentStorage, allowedGlobs []string, deniedGlobs []string) (*SelfDirectedSource, error) {
	return &SelfDirectedSource{
		db:           db,
		allowedGlobs: allowedGlobs,
		deniedGlobs:  deniedGlobs,
	}, nil
}

func (s *SelfDirectedSource) HasCapability(ctx context.Context, userId string, roomId string, capability Capability) (Tristate, error) {
	// Deny wins, so check those first
	for _, g := range s.deniedGlobs {
		if glob.Glob(g, userId) {
			log.Printf("Denied %s from %s the %s capability at glob '%s'", userId, roomId, capability, g)
			return TristateFalse, nil
		}
	}

	for _, g := range s.allowedGlobs {
		if glob.Glob(g, userId) {
			log.Printf("Allowed %s from %s the %s capability at glob '%s'", userId, roomId, capability, g)
			return TristateTrue, nil
		}
	}

	return TristateDefault, nil
}
