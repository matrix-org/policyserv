package test

import (
	"context"
	"sync"
	"testing"

	"github.com/matrix-org/policyserv/pubsub"
	"github.com/stretchr/testify/assert"
)

type MemoryPubsub struct {
	t             *testing.T
	subscriptions map[string][]chan string
	lock          sync.Mutex
}

func NewMemoryPubsub(t *testing.T) *MemoryPubsub {
	return &MemoryPubsub{
		t:             t,
		subscriptions: make(map[string][]chan string),
		lock:          sync.Mutex{},
	}
}

func (m *MemoryPubsub) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, subscription := range m.subscriptions {
		for _, sub := range subscription {
			m.publishTo(sub, pubsub.ClosingValue, true)
		}
	}
	m.subscriptions = make(map[string][]chan string) // clear map
	return nil
}

func (m *MemoryPubsub) publishTo(ch chan string, val string, andClose bool) {
	// Async to avoid blocking calling code
	go func(ch chan string, val string, andClose bool) {
		ch <- val
		if andClose {
			close(ch)
		}
	}(ch, val, andClose)
}

func (m *MemoryPubsub) Publish(ctx context.Context, topic string, val string) error {
	assert.NotNil(m.t, ctx, "context is required")
	assert.NotEmpty(m.t, topic, "topic is required")

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, subscription := range m.subscriptions[topic] {
		m.publishTo(subscription, val, false)
	}

	return nil
}

func (m *MemoryPubsub) Subscribe(ctx context.Context, topic string) (<-chan string, error) {
	assert.NotNil(m.t, ctx, "context is required")
	assert.NotEmpty(m.t, topic, "topic is required")

	m.lock.Lock()
	defer m.lock.Unlock()

	ch := make(chan string)
	if _, ok := m.subscriptions[topic]; !ok {
		m.subscriptions[topic] = make([]chan string, 0)
	}
	m.subscriptions[topic] = append(m.subscriptions[topic], ch)
	return ch, nil
}

func (m *MemoryPubsub) Unsubscribe(ctx context.Context, ch <-chan string) error {
	assert.NotNil(m.t, ctx, "context is required")
	assert.NotNil(m.t, ch, "ch is required")

	m.lock.Lock()
	defer m.lock.Unlock()

	for topic, subs := range m.subscriptions {
		for i, ch2 := range subs {
			if ch == ch2 {
				go func(toClose chan string) {
					ch2 <- pubsub.ClosingValue
					close(toClose)
				}(ch2)
				m.subscriptions[topic] = append(subs[:i], subs[i+1:]...)
				return nil
			}
		}
	}

	return nil
}
