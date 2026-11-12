package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestGptOssSafeguard(t *testing.T) {
	t.Parallel()

	// TODO: A real test, not just a makeshift `main()` function

	provider, err := NewGptOssSafeguard(&config.InstanceConfig{})
	assert.NoError(t, err)
	assert.NotNil(t, provider)

	ret, err := provider.CheckEvent(context.Background(), &GptOssSafeguardConfig{}, &Input{
		Event: test.MustMakePDU(&test.BaseClientEvent{
			RoomId:  "!example:example.org",
			EventId: "$aeRxICtGQzy5TH7k6QQzV8k8lxEVYui6NKy-ubJmVeg",
			Type:    "m.room.message",
			Sender:  "@user:example.org",
			Content: map[string]any{
				"msgtype": "m.text",
				"body":    "hello world",
			},
		}),
	})
	assert.NoError(t, err)
	fmt.Println(ret)

	ret, err = provider.CheckEvent(context.Background(), &GptOssSafeguardConfig{}, &Input{
		Event: test.MustMakePDU(&test.BaseClientEvent{
			RoomId:  "!example:example.org",
			EventId: "$aeRxICtGQzy5TH7k6QQzV8k8lxEVYui6NKy-ubJmVeg",
			Type:    "m.room.message",
			Sender:  "@user:example.org",
			Content: map[string]any{
				"msgtype": "m.text",
				"body":    "You could be rich like me. JOIN https://t.me/redacted TO EARN MONEY NOW",
			},
		}),
	})
	assert.NoError(t, err)
	fmt.Println(ret)
}
