package pubsub

import "context"

// ClosingValue - Sent over a subscribe channel when its closing
const ClosingValue = "<<CLOSING>>"

type Client interface {
	Close() error

	Publish(ctx context.Context, topic string, val string) error
	Subscribe(ctx context.Context, topic string) (<-chan string, error)
	Unsubscribe(ctx context.Context, ch <-chan string) error
}
