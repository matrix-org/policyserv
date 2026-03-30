package api

import (
	"io"
	"log"
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

func httpCheckTextCommunityApi(api *Api, community *storage.StoredCommunity, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpCheckTextCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpCheckTextCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpCheckTextCommunityApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	set, err := api.communityManager.GetFilterSetForCommunityId(r.Context(), community.CommunityId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}
	textToCheck := string(b)

	vecs, err := set.CheckText(r.Context(), textToCheck)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	if set.IsSpamResponse(r.Context(), vecs) {
		// TODO: Also disclose MSC4387 harms, somehow
		errs.text(http.StatusBadRequest, "ORG.MATRIX.MSC4387_SAFETY", "Text is probably spammy")
	} else {
		err = respondJson("httpCheckTextCommunityApi", r, w, make(map[string]any))
		if err != nil {
			errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		}
	}
}

func httpCheckEventIdCommunityApi(api *Api, community *storage.StoredCommunity, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpCheckEventIdCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpCheckEventIdCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpCheckEventIdCommunityApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	var body struct {
		EventId string `json:"event_id"`
	}
	err := parseJsonBody(&body, r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}

	// TODO: Ideally, we'd ensure that the event being requested belongs to the community requesting it.
	// We don't track that information though, so we can't make that determination.

	// See if we already have that event ID
	event, err := api.storage.GetEventResult(r.Context(), body.EventId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
	if event != nil {
		renderEventResult(event.IsProbablySpam, w, r, errs)
		return
	}

	// We don't already have an event - try to fetch it before checking it
	log.Printf("[%s] Fetching event for scan", body.EventId)
	pdu, err := api.hs.GetEvent(r.Context(), body.EventId, api.eventFetchServers)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	// Now check that event
	log.Printf("[%s | %s] Running filters", pdu.EventID(), pdu.RoomID().String())
	ch := make(chan *queue.PoolResult, 1) // buffer to reduce deadlocks
	defer close(ch)
	err = api.hs.RunFilters(r.Context(), pdu, ch)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	// Wait until a result or request timeout
	var res *queue.PoolResult
	select {
	case res = <-ch:
	case <-r.Context().Done():
		log.Printf("[%s | %s] Request context cancelled: %s", pdu.EventID(), pdu.RoomID().String(), r.Context().Err())
		errs.text(http.StatusRequestTimeout, "M_UNKNOWN", "Request timed out")
		return
	}
	if res.Err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", res.Err)
		return
	}
	renderEventResult(res.IsProbablySpam, w, r, errs)
}

func renderEventResult(isProbablySpam bool, w http.ResponseWriter, r *http.Request, errs *errorResponder) {
	if isProbablySpam {
		errs.text(http.StatusBadRequest, "M_FORBIDDEN", "This message is not allowed by the policy server")
	} else {
		err := respondJson("httpCheckEventIdCommunityApi", r, w, map[string]any{})
		if err != nil {
			errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		}
	}
}
