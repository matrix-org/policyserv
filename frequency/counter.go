package frequency

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

type record struct {
	Entity    string    `json:"entity"`
	Timestamp time.Time `json:"timestamp"`
}

type Counter struct {
	pubsub     pubsub.Client
	pubsubId   string
	pubsubStop func()
	lock       *sync.Mutex
	values     map[string][]time.Time // entity -> [timestamp], each record is a +1 count
	window     time.Duration
	ticker     *time.Ticker
}

// NewCounter creates a new cross-process counter. The `name` must be less than 31 characters.
func NewCounter(pubsub pubsub.Client, name string, window time.Duration) (*Counter, error) {
	if len(name) >= 31 {
		return nil, fmt.Errorf("name must be less than 31 characters")
	}
	// This must be less than 64 characters.
	// 	+30 from caller-supplied name
	// 	+27 from KSUID (storage.NextId())
	//  +5  from our other templating
	//  =61
	pubsubId := fmt.Sprintf("ctr.%s.%s", name, storage.NextId())
	if len(pubsubId) > 63 {
		return nil, fmt.Errorf("developer error: pubsub counter ID must be 63 characters or less: %s", pubsubId)
	}
	counter := &Counter{
		pubsub:   pubsub,
		pubsubId: pubsubId,
		lock:     new(sync.Mutex),
		values:   make(map[string][]time.Time),
		window:   window,
		ticker:   time.NewTicker(window),
	}
	return counter, counter.start()
}

func (c *Counter) start() error {
	ch, err := c.pubsub.Subscribe(context.Background(), c.pubsubId)
	if err != nil {
		return err
	}
	log.Printf("Starting %s", c.pubsubId)

	// We start two loops: the first reads from the pubsub channel and the other cleans up entries we don't need
	// anymore.

	go func(ch <-chan string, c *Counter) {
		c.pubsubStop = func() {
			err := c.pubsub.Unsubscribe(context.Background(), ch)
			if err != nil {
				log.Printf("Failed to unsubscribe from %s: %s", c.pubsubId, err)
			}
		}
		for {
			select {
			case val, ok := <-ch:
				if !ok || val == pubsub.ClosingValue {
					return // closed
				}
				rec := record{}
				err := json.Unmarshal([]byte(val), &rec)
				if err != nil {
					log.Printf("Failed to unmarshal value `%s` on %s: %s", val, c.pubsubId, err)
					continue
				}
				c.lock.Lock()
				arr, ok := c.values[rec.Entity]
				if !ok {
					arr = make([]time.Time, 0)
				}
				c.values[rec.Entity] = append(arr, rec.Timestamp)
				log.Printf("Incremented %s (at %s) on %s", rec.Entity, rec.Timestamp.Format(time.RFC1123Z), c.pubsubId)
				c.lock.Unlock()
			}
		}
	}(ch, c)

	go func(c *Counter) {
		for {
			select {
			case _, ok := <-c.ticker.C:
				if !ok {
					return // closed
				}
				log.Printf("Cleaning up %s", c.pubsubId)
				c.lock.Lock()
				for k, v := range c.values {
					newVals := make([]time.Time, 0)
					for _, timestamp := range v {
						if time.Since(timestamp) < c.window {
							newVals = append(newVals, timestamp)
						}
					}
					if len(newVals) == 0 {
						delete(c.values, k)
					} else {
						c.values[k] = newVals
					}
				}
				c.lock.Unlock()
			}
		}
	}(c)

	return nil
}

func (c *Counter) Close() error {
	c.ticker.Stop()
	if c.pubsubStop != nil {
		c.pubsubStop()
	}
	return nil
}

func (c *Counter) Increment(entity string) error {
	rec := record{
		Entity:    entity,
		Timestamp: time.Now(),
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	// We'll pick up our echo on the subscribe
	return c.pubsub.Publish(context.Background(), c.pubsubId, string(b))
}

func (c *Counter) Get(entity string) (int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	vals, ok := c.values[entity]
	if !ok {
		return 0, nil
	}

	// We double check expiration because the cleanup job might not have run yet
	count := 0
	for _, timestamp := range vals {
		if time.Since(timestamp) < c.window {
			count++
		}
	}
	return count, nil
}
