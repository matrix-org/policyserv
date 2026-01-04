package filter

import (
	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/internal"
)

const GptOssSafeguardFilterName = "GptOssSafeguardFilter"

func init() {
	mustRegister(GptOssSafeguardFilterName, &GptOssSafeguardFilter{})
}

type GptOssSafeguardFilter struct {
}

func (g *GptOssSafeguardFilter) MakeFor(set *Set) (Instanced, error) {
	provider, err := ai.NewGptOssSafeguard(set.instanceConfig)
	if err != nil {
		return nil, err
	}
	providerConfig := &ai.GptOssSafeguardConfig{
		FailSecure: internal.Dereference(set.communityConfig.GptOssSafeguardFilterFailSecure),
	}
	instanced := NewInstancedAIExecutorFilter(GptOssSafeguardFilterName, set, providerConfig, provider, set.instanceConfig.GptOssSafeguardAllowedRoomIds)
	return instanced, nil
}
