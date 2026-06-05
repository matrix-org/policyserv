package event

import (
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestExtractHtmlRepresentationsNoHtml(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "doesn't matter",
			"msgtype": "doesn't matter",
			// note: lack of formatted body is what we're testing
		},
	})

	html, err := ExtractHtmlRepresentations(event)
	assert.NoError(t, err)
	assert.Nil(t, html)
}

func TestExtractHtmlRepresentationsWrongEventType(t *testing.T) {
	t.Parallel()

	// This test is temporary until extensible events support exists (at which point we shouldn't be checking types
	// necessarily)

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "org.example.wrong_event_type_goes_here",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":           "doesn't matter",
			"msgtype":        "doesn't matter",
			"format":         "doesn't matter",
			"formatted_body": "<p>doesn't matter</p> <b>for this test</b>",
		},
	})

	html, err := ExtractHtmlRepresentations(event)
	assert.NoError(t, err)
	assert.Nil(t, html)
}

func TestExtractHtmlRepresentations(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":           "doesn't matter",
			"msgtype":        "doesn't matter",
			"format":         "doesn't matter",
			"formatted_body": "<p>this should be extracted</p>",
		},
	})

	html, err := ExtractHtmlRepresentations(event)
	assert.NoError(t, err)
	assert.NotNil(t, html)
	assert.Equal(t, []string{"<p>this should be extracted</p>"}, html)
}
