package harms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentClassString(t *testing.T) {
	assert.Equal(t, "Neutral", ContentClassNeutral.String())
	assert.Equal(t, "Prohibited", ContentClassProhibited.String())
	assert.Equal(t, "Allowed", ContentClassAllowed.String())
}

//goland:noinspection GoBoolExpressions (GoLand wants to simplify these tests to assert.True(t, true), which isn't helpful)
func TestContentClassOrder(t *testing.T) {
	assert.True(t, ContentClassNeutral < ContentClassAllowed)
	assert.True(t, ContentClassAllowed < ContentClassProhibited)
}

func TestProhibitedContent(t *testing.T) {
	c := ProhibitedContent()
	assert.NotNil(t, c)
	assert.Equal(t, []Harm{OtherGeneral}, c.Harms()) // should default to OtherGeneral
	assert.Equal(t, ContentClassProhibited, c.Class())

	c = ProhibitedContent(SpamFraud, PolicyservMedia)
	assert.NotNil(t, c)
	assert.Equal(t, []Harm{SpamFraud, PolicyservMedia}, c.Harms())
	assert.Equal(t, ContentClassProhibited, c.Class())
}

func TestNeutralContent(t *testing.T) {
	c := NeutralContent()
	assert.NotNil(t, c)
	assert.Equal(t, make([]Harm, 0), c.Harms())
	assert.Equal(t, ContentClassNeutral, c.Class())
}

func TestAllowedContent(t *testing.T) {
	c := AllowedContent()
	assert.NotNil(t, c)
	assert.Equal(t, make([]Harm, 0), c.Harms())
	assert.Equal(t, ContentClassAllowed, c.Class())
}

func TestContentInfo(t *testing.T) {
	c := &ContentInfo{} // zero value testing
	assert.Equal(t, ContentClassNeutral, c.Class())
	assert.Equal(t, make([]Harm, 0), c.Harms())

	c = NewContentInfo(ContentClassAllowed)
	assert.Equal(t, ContentClassAllowed, c.Class())
	assert.Equal(t, make([]Harm, 0), c.Harms())

	c = NewContentInfo(ContentClassAllowed, SpamFlooding)
	assert.Equal(t, ContentClassProhibited, c.Class()) // overwritten because we provided a harm to the constructor
	assert.Equal(t, []Harm{SpamFlooding}, c.Harms())

	c = NewContentInfo(ContentClassProhibited)
	assert.Equal(t, ContentClassProhibited, c.Class())
	assert.Equal(t, []Harm{OtherGeneral}, c.Harms()) // default when no harms are specified
}

func TestHarmsDeduplicated(t *testing.T) {
	assert.Equal(t, []Harm{SpamGeneral, SpamFlooding}, deduplicateHarms([]Harm{SpamFlooding, SpamGeneral, SpamFlooding, SpamFlooding, SpamGeneral}))

	// Ensure the constructors are also deduplicating
	assert.Equal(t, []Harm{SpamGeneral, SpamFlooding}, ProhibitedContent(SpamFlooding, SpamGeneral, SpamFlooding, SpamFlooding, SpamGeneral).Harms())
	assert.Equal(t, []Harm{SpamGeneral, SpamFlooding}, NewContentInfo(ContentClassProhibited, SpamFlooding, SpamGeneral, SpamFlooding, SpamGeneral).Harms())
}
