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
		c.Value = ""
		c.Type = ""
		return fmt.Errorf("invalid support contact value: %s", value)
	}

	c.Value = value

	if value[0] == '@' {
		c.Type = SupportContactTypeMatrixUserId
	} else {
		c.Type = SupportContactTypeEmail
	}

	return nil
}
