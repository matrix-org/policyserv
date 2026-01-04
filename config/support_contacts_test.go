package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportContactDecode(t *testing.T) {
	t.Parallel()

	email := "user@example.org"
	userId := "@user:example.org"
	neither := "user_example.org"

	val := &SupportContact{}

	assert.NoError(t, val.Decode(email))
	assert.Equal(t, email, val.Value)
	assert.Equal(t, SupportContactTypeEmail, val.Type)

	assert.NoError(t, val.Decode(userId))
	assert.Equal(t, userId, val.Value)
	assert.Equal(t, SupportContactTypeMatrixUserId, val.Type)

	err := val.Decode(neither)
	assert.EqualError(t, err, "invalid support contact value: "+neither)
	assert.Empty(t, val.Value)
	assert.Empty(t, val.Type)
}
