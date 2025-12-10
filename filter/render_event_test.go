package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestRenderEventMText(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "hello world",
			"msgtype": "m.text",
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: hello world"}, render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":           "hello world",
			"msgtype":        "m.text",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>hello world</b>",
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: hello world", "@alice:example.org says: <b>hello world</b>"}, render)
}

func TestRenderEventMNotice(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "hello world",
			"msgtype": "m.notice",
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: hello world"}, render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":           "hello world",
			"msgtype":        "m.notice",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>hello world</b>",
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: hello world", "@alice:example.org says: <b>hello world</b>"}, render)
}

func TestRenderEventMEmote(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "waves hello",
			"msgtype": "m.emote",
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: /me waves hello"}, render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":           "waves hello",
			"msgtype":        "m.emote",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>waves hello</b>",
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org says: /me waves hello", "@alice:example.org says: /me <b>waves hello</b>"}, render)
}

func TestRenderEventUnrenderableMessages(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.image", // not a supported message type
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.video", // not a supported message type
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.audio", // not a supported message type
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.file", // not a supported message type
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)

	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "org.example.custom", // not a supported message type
		},
	})
	render, err = renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)
}

func TestRenderEventReaction(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.reaction",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"m.relates_to": map[string]any{
				"rel_type": "m.annotation",
				"key":      "💖",
				"event_id": "$test1",
			},
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string{"@alice:example.org reacted with 💖"}, render)
}

func TestRenderEventUnknownType(t *testing.T) {
	t.Parallel()

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "org.example.some_unknown_event_type",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"this": "does not matter",
		},
	})
	render, err := renderEventToText(event)
	assert.NoError(t, err)
	assert.Equal(t, []string(nil), render)
}
