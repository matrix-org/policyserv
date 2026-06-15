package filter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/notifiers"
)

type auditContext struct {
	Event           gomatrixserverlib.PDU
	IsSpam          bool
	FilterResponses map[string][]string
	CommunityId     string

	lock     sync.Mutex // use a lock instead of a sync.Map because sync.Map doesn't support generics (and library support appears lacking in quality)
	notifier notifiers.MatrixNotifier
}

func newAuditContext(notifier notifiers.MatrixNotifier, communityId string, event gomatrixserverlib.PDU) (*auditContext, error) {
	return &auditContext{
		Event:           event,
		FilterResponses: make(map[string][]string),
		CommunityId:     communityId,

		// Populated later
		IsSpam: false,

		// Internal
		lock:     sync.Mutex{},
		notifier: notifier,
	}, nil
}

func (c *auditContext) AppendFilterResponse(filterName string, contentInfo *harms.ContentInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.FilterResponses[filterName] = []string{contentInfo.Class().String()}
	for _, h := range contentInfo.Harms() {
		c.FilterResponses[filterName] = append(c.FilterResponses[filterName], string(h))
	}
}

func (c *auditContext) Publish() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Note: we log the audit context so if the webhook fails (or isn't configured) then we
	// have an idea of what happened.
	log.Printf("[%s | %s | %s] Audit publish: %#v", c.Event.EventID(), c.Event.RoomID(), c.Event.SenderID(), c)

	if !c.IsSpam {
		return nil // nothing to publish
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

	wasHellban := c.FilterResponses[HellbanPrefilterName] != nil && slices.Contains(c.FilterResponses[HellbanPrefilterName], string(harms.SpamGeneral))

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

	// we don't html2text this because long events can cause hookshot to only show text versions, making all
	// of our work to contain the spam to a <details> block useless. We still put some sort of message here
	// though so clients which don't support HTML can still see something useful.
	textAudit := "This event requires HTML."

	msgId, err := c.notifier.Send(c.CommunityId, textAudit, htmlAudit)
	if err != nil {
		return fmt.Errorf("failed to send audit message: %w", err)
	}
	log.Printf("[%s | %s | %s] Audit message sent: %s", c.Event.EventID(), c.Event.RoomID(), c.Event.SenderID(), msgId)
	return nil
}
