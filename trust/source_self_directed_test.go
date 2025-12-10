package trust

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestSelfDirectedSource(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewSelfDirectedSource(db, []string{"@allowed:*"}, []string{"@denied:*"})
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// Check that a trusted user is in fact trusted
	res, err := source.HasCapability(context.Background(), "@allowed:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateTrue, res)

	// and that untrusted users are explicitly denied
	res, err = source.HasCapability(context.Background(), "@denied:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateFalse, res)

	// and that everyone else causes no opinion
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)
}

func TestSelfDirectedSourcePrioritizesDeny(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewSelfDirectedSource(db, []string{"@user:*", "@allowed:*"}, []string{"@user:*"})
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// Verify we are at least checking the allowed users list so we can be sure the deny is actually working
	res, err := source.HasCapability(context.Background(), "@allowed:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateTrue, res)

	// Now check that the user is denied
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateFalse, res)
}
