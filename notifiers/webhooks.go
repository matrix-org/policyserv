package notifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/panjf2000/ants/v2"
)

var ErrWebhookNotHttps = errors.New("webhook must be https")
var ErrWebhookDomainNotAllowed = errors.New("webhook domain not allowed")

type WebhookMatrixNotifier struct {
	MatrixNotifier

	storage        storage.PersistentStorage
	pool           *ants.Pool
	allowedDomains []string
}

func NewWebhookMatrixNotifier(storage storage.PersistentStorage, poolSize int, allowedDomains []string) (*WebhookMatrixNotifier, error) {
	pool, err := ants.NewPool(poolSize, ants.WithOptions(ants.Options{
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
	return &WebhookMatrixNotifier{
		storage:        storage,
		pool:           pool,
		allowedDomains: allowedDomains,
	}, nil
}

func (w *WebhookMatrixNotifier) Send(communityId string, plainText string, htmlText string) (string, error) {
	// This context only covers database calls and queue setup - it's not used for actual delivery
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	community, err := w.storage.GetCommunity(ctx, communityId)
	if err != nil {
		return "", err
	}
	target := internal.Dereference(community.Config.WebhookUrl)
	if target == "" {
		msgId := storage.NextId()
		log.Printf("[%s] No webhook configured for community %s", msgId, communityId)
		return msgId, nil
	}

	whUrl, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	if !testing.Testing() {
		if whUrl.Scheme != "https" {
			return "", ErrWebhookNotHttps
		}
	}
	if !slices.Contains(w.allowedDomains, whUrl.Host) {
		return "", ErrWebhookDomainNotAllowed
	}

	msgId := storage.NextId()
	err = w.pool.Submit(func() {
		// We override the context here to ensure we don't spend forever trying to send a message
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Hookshot / Slack format body
		reqBody := make(map[string]any)
		reqBody["text"] = plainText
		reqBody["html"] = htmlText

		buf := bytes.NewBuffer(nil)
		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false) // we expect XSS protection from elsewhere, so avoid making the JSON hard to read by humans/tests
		err := encoder.Encode(reqBody)
		if err != nil {
			log.Printf("[%s] Failed to encode JSON: %s", msgId, err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, whUrl.String(), buf)
		if err != nil {
			log.Printf("[%s] Failed to create request: %s", msgId, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "policyserv")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[%s] Failed to send request: %s", msgId, err)
			return
		}
		defer res.Body.Close()
		log.Printf("[%s] Webhook response: %s", msgId, res.Status)
	})
	return msgId, err
}
