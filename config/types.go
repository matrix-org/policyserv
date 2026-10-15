package config

import (
	"fmt"

	"github.com/openai/openai-go/v3/shared"
)

type GptOssSafeguardReasoningEffort shared.ReasoningEffort // Implements envconfig.Decoder

func (e *GptOssSafeguardReasoningEffort) Decode(value string) error {
	switch value {
	case "":
		fallthrough
	case "low":
		*e = GptOssSafeguardReasoningEffort(shared.ReasoningEffortLow)
		return nil
	case "medium":
		*e = GptOssSafeguardReasoningEffort(shared.ReasoningEffortMedium)
		return nil
	case "high":
		*e = GptOssSafeguardReasoningEffort(shared.ReasoningEffortHigh)
		return nil
	}

	return fmt.Errorf("unsupported reasoning effort '%s'", value)
}
