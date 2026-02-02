package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/require"
)

func TestLinkFilter_NoListsConfigured(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{},
		denyList:  []string{},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test1",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Check this out: https://example.com/page",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, result, "No lists configured, filter should have no opinion")
}

func TestLinkFilter_NoLinks(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{"https://github.com/*"},
		denyList:  []string{},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test2",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Hello world, no links here!",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, result, "No links in message, filter should have no opinion")
}

func TestLinkFilter_AllowList_Match(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{"https://github.com/*", "https://spec.matrix.org/*"},
		denyList:  []string{},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test3",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Issue: https://github.com/matrix-org/policyserv/issues/123",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, result, "Link matches allow list, should be allowed")
}

func TestLinkFilter_AllowList_NoMatch(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{"https://github.com/*"},
		denyList:  []string{},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test4",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "See: https://nsfw-site.example/bad-stuff",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []classification.Classification{classification.Spam}, result, "Link does not match allow list, should be spam")
}

func TestLinkFilter_DenyList_Match(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{},
		denyList:  []string{"*nsfw-site.example*"},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test5",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Check this: https://nsfw-site.example/path",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []classification.Classification{classification.Spam}, result, "Link matches deny list, should be spam")
}

func TestLinkFilter_DenyList_NoMatch(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{},
		denyList:  []string{"*nsfw-site.example*"},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test6",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Good site: https://example.org/page",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, result, "Link does not match deny list, should be allowed")
}

func TestLinkFilter_BothLists_AllowAndDeny(t *testing.T) {
	// Scenario: AllowList permits github, but DenyList bans a specific subdirectory.
	f := &InstancedLinkFilter{
		allowList: []string{"https://github.com/*"},
		denyList:  []string{"https://github.com/banned-user/*"},
	}

	// Allowed
	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test7a",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "https://github.com/matrix-org/policyserv",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, result, "Link matches allow list and not deny list, should be allowed")

	// Denied
	event = test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test7b",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "https://github.com/banned-user/repo",
			"msgtype": "m.text",
		},
	})
	input = &Input{Event: event}
	result, err = f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []classification.Classification{classification.Spam}, result, "Link matches deny list, should be spam")
}

func TestLinkFilter_MultipleLinks_OneBad(t *testing.T) {
	f := &InstancedLinkFilter{
		allowList: []string{"https://github.com/*"},
		denyList:  []string{},
	}

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$test8",
		RoomId:  "!test:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"body":    "Good: https://github.com/foo Bad: https://evil.com/path",
			"msgtype": "m.text",
		},
	})
	input := &Input{Event: event}
	result, err := f.CheckEvent(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []classification.Classification{classification.Spam}, result, "One link is not on allow list, should be spam")
}
