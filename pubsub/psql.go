package pubsub

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/matrix-org/policyserv/storage"
	"github.com/lib/pq"
)

type PostgresPubsubConnectionConfig struct {
	Uri                  string
	MinReconnectInterval time.Duration
	MaxReconnectInterval time.Duration
}

type PostgresPubsub struct {
	db       *storage.PostgresStorage
	listener *pq.Listener
	lock     sync.Mutex
	channels map[string][]chan string
}

func NewPostgresPubsub(db *storage.PostgresStorage, config *PostgresPubsubConnectionConfig) (*PostgresPubsub, error) {
	listener := pq.NewListener(config.Uri, config.MinReconnectInterval, config.MaxReconnectInterval, func(event pq.ListenerEventType, err error) {
		if err != nil {
			log.Println("Pubsub listener error:", err)
		}
	})
	p := &PostgresPubsub{
		db:       db,
		listener: listener,
		channels: make(map[string][]chan string),
	}
	go func(p *PostgresPubsub) {
		for {
			select {
			case v := <-p.listener.Notify:
				if v == nil {
					continue // likely a reconnect
				}
				log.Printf("Got a notification %#v\n", v)
				chans := p.channels[v.Channel]
				if chans == nil {
					continue // ignore
				}
				for _, ch := range chans {
					ch <- v.Extra
				}
			case <-time.After(30 * time.Second):
				//goland:noinspection GoUnhandledErrorResult
				go p.listener.Ping()
			}
		}
	}(p)
	return p, nil
}

func (p *PostgresPubsub) Close() error {
	for _, chans := range p.channels {
		for _, ch := range chans {
			go func(ch chan string) {
				ch <- ClosingValue
				close(ch)
			}(ch)
		}
	}
	return p.listener.Close()
}

func (p *PostgresPubsub) Publish(ctx context.Context, topic string, val string) error {
	return p.db.SendNotify(ctx, topic, val)
}

func (p *PostgresPubsub) Subscribe(ctx context.Context, topic string) (<-chan string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	ch := make(chan string)

	// Only subscribe to the topic (channel) once.
	if chans, ok := p.channels[topic]; !ok || len(chans) == 0 {
		err := p.listener.Listen(topic)
		if err != nil {
			return nil, err
		}
	}

	p.channels[topic] = append(p.channels[topic], ch)
	return ch, nil
}

func (p *PostgresPubsub) Unsubscribe(ctx context.Context, ch <-chan string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for topic, chans := range p.channels {
		for i, maybeCh := range chans {
			if maybeCh == ch {
				// Use a goroutine to avoid blocking on closure
				go func(toCloseCh chan string) {
					toCloseCh <- ClosingValue
					close(toCloseCh)
				}(maybeCh)
				p.channels[topic] = append(p.channels[topic][:i], p.channels[topic][i+1:]...)
				return nil
			}
		}
	}

	return nil
}
