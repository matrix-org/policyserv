package homeserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

type eventRoomIdOnly struct {
	RoomId string `json:"room_id"`
}

func httpTransactionReceive(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpTransactionReceive")
	t := metrics.StartRequestTimer(r.Method, "httpTransactionReceive")
	defer t.ObserveDuration()

	if r.Method != http.MethodPut {
		defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return
	}

	fedReq, fedErr := fclient.VerifyHTTPRequest(r, time.Now(), server.ServerName, server.isSelf, server.keyRing)
	if !fedErr.Is2xx() {
		b, err := json.Marshal(fedErr.JSON)
		if err != nil {
			log.Println("Error marshalling fedErr:", err)
			defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusInternalServerError)
			MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal error response")
			return
		}

		defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", fedErr.Code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fedErr.Code)
		_, _ = w.Write(b)
		return
	}

	txn := gomatrixserverlib.Transaction{}
	err := json.Unmarshal(fedReq.Content(), &txn)
	if err != nil {
		log.Println("Error unmarshalling fedReq:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to parse transaction")
		return
	}

	// Process each PDU in the transaction in an attempt to queue it for checking
	resp := &fclient.RespSend{
		PDUs: make(map[string]fclient.PDUResult),
	}
	for _, eventRaw := range txn.PDUs {
		// First we need to know the room version so we can parse the event properly. This relies on the room_id.
		header := eventRoomIdOnly{}
		err = json.Unmarshal(eventRaw, &header)
		if err != nil {
			log.Println("Error extracting room ID from event:", err)
			//goland:noinspection GoDeferInLoop
			defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusInternalServerError)
			MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to parse transaction")
			return
		}

		var room *storage.StoredRoom
		room, err = server.storage.GetRoom(r.Context(), header.RoomId)
		if err != nil {
			log.Println("Non-fatal error getting room:", err)
			continue
		}
		if room == nil {
			log.Println("Non-fatal error getting room: room not found")
			continue
		}
		roomVersion := gomatrixserverlib.MustGetRoomVersion(gomatrixserverlib.RoomVersion(room.RoomVersion))

		// Parse the event and verify its signatures
		var event gomatrixserverlib.PDU
		event, err = roomVersion.NewEventFromUntrustedJSON(eventRaw)
		if err != nil {
			log.Println("Non-fatal error parsing event:", err)
			continue
		}
		if err = gomatrixserverlib.VerifyEventSignatures(r.Context(), event, server.keyRing, func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
			return senderId.ToUserID(), nil
		}); err != nil {
			log.Printf("Could not verify signatures of %s - ignoring event. %s", event.EventID(), err.Error())
			resp.PDUs[event.EventID()] = fclient.PDUResult{
				Error: err.Error(),
			}
			continue
		}

		// We've "accepted" the event with no errors, so mark that
		resp.PDUs[event.EventID()] = fclient.PDUResult{}

		if event.Redacted() {
			continue // we ignore redacted events (for now?)
		}

		// *Now* we can queue the event for checking
		// Note: we use a background context instead of request context because the request might finish before the
		// event is run through the filters. We don't want to do that forever though. We also async the check because
		// we don't really need to block the request on each event getting checked individually.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			ch := make(chan *queue.PoolResult, 1) // use a buffered channel to reduce deadlock potential
			defer close(ch)
			err = server.RunFilters(ctx, event, ch)
			if err != nil {
				log.Printf("Error queueing event %s: %s", event.EventID(), err)
				return
			}

			var res *queue.PoolResult
			select {
			case res = <-ch:
			case <-ctx.Done():
				log.Printf("[%s | %s] Request context cancelled: %s", event.EventID(), event.RoomID().String(), r.Context().Err())
				return
			}

			if res.Err != nil {
				log.Printf("[%s | %s] Error processing event: %s", event.EventID(), event.RoomID().String(), res.Err)
				return
			}

			if res.IsProbablySpam {
				redactIfNeeded(ctx, server, "not_a_real_server_to_always_fail_the_included_sender_check", event)
			}
		}()
	}

	// Don't forget to actually reply too
	b, err := json.Marshal(resp)
	if err != nil {
		defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal response")
		return
	}

	defer metrics.RecordHttpResponse(r.Method, "httpTransactionReceive", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
