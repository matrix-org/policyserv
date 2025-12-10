package homeserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/redaction"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

var msc4284NeutralResponse = []byte(`{"recommendation": "ok"}`) // MSC4284 uses the word "ok" to mean neutral.
var msc4284SpamResponse = []byte(`{"recommendation": "spam"}`)

func httpMSC4284Check(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpMSC4284Check")
	t := metrics.StartRequestTimer(r.Method, "httpMSC4284Check")
	defer t.ObserveDuration()

	fedReq, room := decodeRoom("httpMSC4284Check", server, w, r)
	if room == nil {
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(msc4284NeutralResponse)
		return
	}

	roomVersion := gomatrixserverlib.MustGetRoomVersion(gomatrixserverlib.RoomVersion(room.RoomVersion))
	event, err := roomVersion.NewEventFromUntrustedJSON(fedReq.Content())
	if err != nil {
		log.Println("Error parsing event:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to parse event")
		return
	}

	// Verify event signatures before moving any further
	// This replaces the need for a signature-checking filter in the pipeline
	err = gomatrixserverlib.VerifyEventSignatures(r.Context(), event, server.keyRing, func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
		return senderId.ToUserID(), nil
	})
	if err != nil {
		log.Printf("Signature verification failed for %s: %s", event.EventID(), err)
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(msc4284SpamResponse) // it's a bad event, so spammy
		return
	}

	log.Printf("[%s] Asked to check in %s by %s", event.EventID(), event.RoomID().String(), fedReq.Origin())

	ch := make(chan *queue.PoolResult, 1) // use a buffered channel to reduce deadlock potential
	defer close(ch)
	err = server.runFilters(r.Context(), event, ch)
	if err != nil {
		log.Println("Error submitting event:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to start event scan")
		return
	}

	// We don't want to read the result indefinitely, so involve the context
	var res *queue.PoolResult
	select {
	case res = <-ch:
	case <-r.Context().Done():
		log.Printf("[%s | %s] Request context cancelled: %s", event.EventID(), event.RoomID().String(), r.Context().Err())
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusRequestTimeout)
		MatrixHttpError(w, http.StatusRequestTimeout, "M_UNKNOWN", "Request timed out")
		return
	}

	if res.Err != nil {
		log.Println("Error receiving event result:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to complete event scan")
		return
	}

	if res.IsProbablySpam {
		senderDomain := spec.ServerName("example.org")
		if event.SenderID().IsUserID() {
			senderDomain = event.SenderID().ToUserID().Domain()
		}
		if senderDomain != fedReq.Origin() {
			for _, origin := range server.trustedOrigins {
				if fedReq.Origin() == spec.ServerName(origin) {
					err = redaction.QueueRedaction(server.storage, event)
					if err != nil {
						log.Printf("Non-fatal error trying to submit redaction for spammy event %s in %s: %s", event.EventID(), event.RoomID().String(), err)
					}
					break
				}
			}
		}
	}

	defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Check", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if res.IsProbablySpam {
		_, _ = w.Write(msc4284SpamResponse)
	} else {
		_, _ = w.Write(msc4284NeutralResponse)
	}
}

func decodeRoom(name string, server *Homeserver, w http.ResponseWriter, r *http.Request) (*fclient.FederationRequest, *storage.StoredRoom) {
	if r.Method != http.MethodPost {
		defer metrics.RecordHttpResponse(r.Method, name, http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return nil, nil
	}

	fedReq, fedErr := fclient.VerifyHTTPRequest(r, time.Now(), server.ServerName, server.isSelf, server.keyRing)
	if !fedErr.Is2xx() {
		b, err := json.Marshal(fedErr.JSON)
		if err != nil {
			log.Println("Error marshalling fedErr:", err)
			defer metrics.RecordHttpResponse(r.Method, name, http.StatusInternalServerError)
			MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal error response")
			return nil, nil
		}

		defer metrics.RecordHttpResponse(r.Method, name, fedErr.Code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fedErr.Code)
		_, _ = w.Write(b)
		return nil, nil
	}

	header := eventRoomIdOnly{}
	err := json.Unmarshal(fedReq.Content(), &header)
	if err != nil {
		log.Println("Error unmarshalling fedReq:", err)
		defer metrics.RecordHttpResponse(r.Method, name, http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to unmarshal request body header")
		return nil, nil
	}

	room, err := server.storage.GetRoom(r.Context(), header.RoomId)
	if err != nil {
		log.Println("Error getting room:", err)
		defer metrics.RecordHttpResponse(r.Method, name, http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to get room")
		return nil, nil
	}
	if room == nil {
		log.Printf("Room %s is unknown", header.RoomId)
	}
	return fedReq, room
}
