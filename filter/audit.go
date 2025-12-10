package filter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/audit"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/gomatrixserverlib"
)

type auditContext struct {
	Event              gomatrixserverlib.PDU
	IsSpam             bool
	FinalVectors       confidence.Vectors
	IncrementalVectors []confidence.Vectors
	FilterResponses    map[string][]classification.Classification
	WebhookUrl         string

	lock           sync.Mutex // use a lock instead of a sync.Map because sync.Map doesn't support generics (and library support appears lacking in quality)
	instanceConfig *config.InstanceConfig
}

func newAuditContext(instanceConfig *config.InstanceConfig, event gomatrixserverlib.PDU, webhookUrl string) (*auditContext, error) {
	return &auditContext{
		Event:              event,
		FilterResponses:    make(map[string][]classification.Classification),
		IncrementalVectors: make([]confidence.Vectors, 0),
		WebhookUrl:         webhookUrl,

		// Populated later
		IsSpam:       false,
		FinalVectors: nil,

		// Internal
		lock:           sync.Mutex{},
		instanceConfig: instanceConfig,
	}, nil
}

func (c *auditContext) AppendFilterResponse(filterName string, classifications []classification.Classification) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.FilterResponses[filterName] = classifications
}

func (c *auditContext) AppendSetGroupVectors(vectors confidence.Vectors) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.IncrementalVectors = append(c.IncrementalVectors, vectors)
}

func (c *auditContext) Publish(workQueue *audit.Queue) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Note: we log the audit context so if the webhook fails (or isn't configured) then we
	// have an idea of what happened.
	log.Printf("[%s | %s | %s] Audit publish: %#v", c.Event.EventID(), c.Event.RoomID(), c.Event.SenderID(), c)

	if c.WebhookUrl == "" || !c.IsSpam {
		return nil // nothing to publish
	}

	// Validate URL
	whUrl, err := url.Parse(c.WebhookUrl)
	if err != nil {
		return err
	}
	if !testing.Testing() {
		if whUrl.Scheme != "https" {
			return fmt.Errorf("webhook URL must be HTTPS")
		}
	}
	if !slices.Contains(c.instanceConfig.AllowedWebhookDomains, whUrl.Host) {
		return fmt.Errorf("webhook URL host not allowed")
	}

	respsJson, err := json.MarshalIndent(c.FilterResponses, "", "  ")
	if err != nil {
		return err // "should never happen"
	}
	contentBuf := bytes.NewBuffer(nil)
	err = json.Indent(contentBuf, c.Event.Content(), "", "  ")
	if err != nil {
		return err // "should never happen"
	}
	contentJson := contentBuf.String()

	wasHellban := c.FilterResponses[HellbanPrefilterName] != nil && slices.Contains(c.FilterResponses[HellbanPrefilterName], classification.Spam)

	htmlAudit := "A user has had an event of theirs flagged as spam by policyserv:<br/>"
	htmlAudit += fmt.Sprintf("<b>User ID:</b> <code>%s</code><br/>", html.EscapeString(string(c.Event.SenderID())))
	escapedRoomId := html.EscapeString(c.Event.RoomID().String())
	htmlAudit += fmt.Sprintf("<b>Room ID:</b> <code>%s</code> (<a href=\"https://matrix.to/#/%s\">%s</a>)<br/>", escapedRoomId, escapedRoomId, escapedRoomId)
	if wasHellban {
		htmlAudit += "<b>This was a hellban. More details are below.</b><br/><details><summary>Event info (click to expand)</summary>"
	}
	htmlAudit += fmt.Sprintf("<b>Event ID:</b> <code>%s</code><br/><i>The event ID may not exist if the origin server rejected it due to spam.</i><br/>", html.EscapeString(c.Event.EventID()))
	htmlAudit += fmt.Sprintf("<b>Event type:</b> <code>%s</code><br/>", html.EscapeString(c.Event.Type()))
	htmlAudit += fmt.Sprintf("<b>Event timestamp:</b> %s<br/>", c.Event.OriginServerTS().Time().Format(time.RFC1123Z))
	htmlAudit += fmt.Sprintf("<b>Recorded time:</b> %s<br/>", time.Now().Format(time.RFC1123Z))
	htmlAudit += fmt.Sprintf("<details><summary>Filter responses (click to expand)</summary><pre><code>%s</code></pre></details>", html.EscapeString(string(respsJson)))
	htmlAudit += fmt.Sprintf("<details><summary>Event content (%d bytes; click to expand)</summary><pre><code>%s</code></pre></details>", len(contentJson), html.EscapeString(contentJson))
	if wasHellban {
		htmlAudit += "</details>" // close the details block from earlier
	}

	return workQueue.Submit(c.Event.EventID(), htmlAudit, whUrl.String())
}
