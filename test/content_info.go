package test

import (
	"testing"

	"github.com/matrix-org/policyserv/harms"
	"github.com/stretchr/testify/assert"
)

func AssertEqualContentInfo(t *testing.T, expected *harms.ContentInfo, actual *harms.ContentInfo) {
	assert.NotNil(t, expected)
	assert.NotNil(t, actual)
	assert.Equal(t, expected.Class().String(), actual.Class().String()) // compare strings to make debugging easier
	assert.Equal(t, expected.Harms(), actual.Harms())
}
