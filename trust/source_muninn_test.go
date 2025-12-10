package trust

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestMuninnHallSource(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewMuninnHallSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// No data == no opinion
	res, err := source.HasCapability(context.Background(), "@user:example.org", "!ignored", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)

	// Add some data for ease of testing
	err = source.ImportData(context.Background(), MuninnHallMemberDirectory{
		"example.org": {"@admin:example.org"},
	})
	assert.NoError(t, err)

	// Now check that the domain is trusted
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!ignored", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateTrue, res)

	// ... and that other domains are not (by returning no opinion)
	res, err = source.HasCapability(context.Background(), "@user:untrusted.example.org", "!ignored", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)
}
