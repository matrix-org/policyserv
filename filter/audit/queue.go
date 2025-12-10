package audit

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/panjf2000/ants/v2"
)

type Queue struct {
	pool *ants.Pool
}

func NewQueue(size int) (*Queue, error) {
	pool, err := ants.NewPool(size, ants.WithOptions(ants.Options{
		// Same options as the queue.Pool setup
		ExpiryDuration:   1 * time.Minute,
		PreAlloc:         false,
		MaxBlockingTasks: 0, // no limit on submissions
		Nonblocking:      false,
		// If we don't supply a panic handler then ants will print a stack trace for us
		//PanicHandler: func(err interface{}) {
		//	log.Println("Panic in pool:", err)
		//},
		Logger:       log.Default(),
		DisablePurge: false,
	}))
	if err != nil {
		return nil, err
	}
	return &Queue{
		pool: pool,
	}, nil
}

func (q *Queue) Submit(eventId string, html string, webhookUrl string) error {
	workFn := func() {
		// Hookshot / Slack format body
		reqBody := make(map[string]any)
		reqBody["html"] = html

		// we don't html2text this because long events can cause hookshot to only show text versions, making all
		// of our work to contain the spam to a <details> block useless. We still put some sort of message here
		// though so clients which don't support HTML can still see something useful.
		reqBody["text"] = "This event requires HTML."

		buf := bytes.NewBuffer(nil)
		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false) // we get XSS protection from elsewhere, so avoid making the JSON hard to read by humans/tests
		err := encoder.Encode(reqBody)
		if err != nil {
			log.Printf("[%s] Failed to encode JSON: %s", eventId, err)
			return
		}

		req, err := http.NewRequest("POST", webhookUrl, buf)
		if err != nil {
			log.Printf("[%s] Failed to create request: %s", eventId, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "policyserv")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[%s] Failed to send request: %s", eventId, err)
			return
		}
		defer res.Body.Close()
		log.Printf("[%s] Audit webhook response: %s", eventId, res.Status)
	}
	return q.pool.Submit(workFn)
}
