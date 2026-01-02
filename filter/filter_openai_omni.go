package filter

import (
	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/internal"
)

const OpenAIOmniFilterName = "OpenAIOmniFilter"

func init() {
	mustRegister(OpenAIOmniFilterName, &OpenAIOmniFilter{})
}

type OpenAIOmniFilter struct {
}

func (o *OpenAIOmniFilter) MakeFor(set *Set) (Instanced, error) {
	provider, err := ai.NewOpenAIOmniModeration(set.instanceConfig)
	if err != nil {
		return nil, err
	}
	providerConfig := &ai.OpenAIOmniModerationConfig{
		FailSecure: internal.Dereference(set.communityConfig.OpenAIFilterFailSecure),
	}
	instanced := NewInstancedAIExecutorFilter(OpenAIOmniFilterName, set, providerConfig, provider, set.instanceConfig.OpenAIAllowedRoomIds)
	return instanced, nil
}
