package config

import (
	"fmt"
	"strings"
)

type SupportContactType string

const SupportContactTypeEmail SupportContactType = "email"
const SupportContactTypeMatrixUserId SupportContactType = "matrix_user_id"

type SupportContact struct {
	Value string
	Type  SupportContactType
}

func (c *SupportContact) Decode(value string) error {
	// Implements envconfig.Decoder

	if !strings.Contains(value, "@") {
		*c = SupportContact{
			Value: "",
			Type:  "",
		}
		return fmt.Errorf("invalid support contact value: %s", value)
	}

	contactType := SupportContactTypeEmail
	if value[0] == '@' {
		contactType = SupportContactTypeMatrixUserId
	}
	*c = SupportContact{
		Value: value,
		Type:  contactType,
	}

	return nil
}
