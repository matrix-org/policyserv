package homeserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/queue"
)

const PolicyServerKeyID gomatrixserverlib.KeyID = "ed25519:policy_server"

var msc4284NoSignature = []byte(`{}`)

type signatures struct {
	Signatures map[string]map[gomatrixserverlib.KeyID]spec.Base64Bytes `json:"signatures"`
}

func httpMSC4284Sign(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpMSC4284Sign")
	t := metrics.StartRequestTimer(r.Method, "httpMSC4284Sign")
	defer t.ObserveDuration()

	handlePolicySignRequest(server, w, r, false)
}

func handlePolicySignRequest(server *Homeserver, w http.ResponseWriter, r *http.Request, stable bool) {
	funcName := "httpMSC4284Sign"
	if stable {
		funcName = "httpPolicySign"
	}
	fedReq, room := decodeRoom(funcName, server, w, r)
	if room == nil {
		// we must have a room_id to know if we should sign it.
		// Notably the create event in v12 rooms will omit this.
		refuseToSign(w, r, stable)
		return
	}

	roomVersion := gomatrixserverlib.MustGetRoomVersion(gomatrixserverlib.RoomVersion(room.RoomVersion))
	event, err := roomVersion.NewEventFromUntrustedJSON(fedReq.Content())
	if err != nil {
		log.Println("Error parsing event:", err)
		refuseToSign(w, r, stable)
		return
	}

	// Verify event signatures before moving any further
	// This replaces the need for a signature-checking filter in the pipeline
	err = gomatrixserverlib.VerifyEventSignatures(r.Context(), event, server.keyRing, func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
		return senderId.ToUserID(), nil
	})
	if err != nil {
		log.Printf("Signature verification failed for %s: %s", event.EventID(), err)
		refuseToSign(w, r, stable)
		return
	}

	log.Printf("🔏 [%s] asked to sign in %s by %s", event.EventID(), event.RoomID().String(), fedReq.Origin())

	ch := make(chan *queue.PoolResult, 1) // use a buffered channel to reduce deadlock potential
	defer close(ch)
	err = server.RunFilters(r.Context(), event, ch)
	if err != nil {
		log.Println("Error submitting event:", err)
		refuseToSign(w, r, stable)
		redactIfNeeded(r.Context(), server, fedReq.Origin(), event)
		return
	}

	// We don't want to read the result indefinitely, so involve the context
	var res *queue.PoolResult
	select {
	case res = <-ch:
	case <-r.Context().Done():
		log.Printf("[%s | %s] Request context cancelled: %s", event.EventID(), event.RoomID().String(), r.Context().Err())
		defer metrics.RecordHttpResponse(r.Method, funcName, http.StatusRequestTimeout)
		MatrixHttpError(w, http.StatusRequestTimeout, "M_UNKNOWN", "Request timed out")
		return
	}

	if res.Err != nil {
		log.Println("Error receiving event result:", err)
		refuseToSign(w, r, stable)
		redactIfNeeded(r.Context(), server, fedReq.Origin(), event)
		return
	}

	if res.IsProbablySpam {
		log.Printf("🚫 [%s] refusing to sign in %s", event.EventID(), event.RoomID().String())
		refuseToSign(w, r, stable)
		redactIfNeeded(r.Context(), server, fedReq.Origin(), event)
		return
	}

	signEvent(server, event, w)
	log.Printf("✅ [%s] Signed in %s as requested by %s", event.EventID(), event.RoomID().String(), fedReq.Origin())
}

func redactIfNeeded(ctx context.Context, server *Homeserver, requestOrigin spec.ServerName, event gomatrixserverlib.PDU) {
	// We need to be a bit careful with invalid user IDs here. If things are invalid, we'll try to redact the event
	// as a precaution.
	senderDomain := spec.ServerName("undefined.invalid")
	if event.SenderID().IsUserID() && event.SenderID().ToUserID() != nil {
		senderDomain = event.SenderID().ToUserID().Domain()
	}

	// If the sender and requesting domain are the same, we're assuming that the server will reject the event. If not,
	// then we'll either see the event over `/send` (where the requestOrigin will be nil-like) or from another server
	// re-checking the event. *Then* we'll redact it.
	if senderDomain == requestOrigin {
		log.Printf("[%s | %s] Sender and requesting domain are the same - assuming they rejected the event.", event.EventID(), event.RoomID().String())
		return
	}

	err := server.SendRedactInstruction(ctx, event)
	if err != nil {
		log.Printf("[%s | %s] Non-fatal error trying to submit redaction for event", event.EventID(), event.RoomID().String())
	}
}

func refuseToSign(w http.ResponseWriter, r *http.Request, stable bool) {
	// Stable endpoints allow us to return real errors, so we should do that.
	if stable {
		MatrixHttpError(w, http.StatusBadRequest, "M_FORBIDDEN", "This message is not allowed by the policy server")
		return
	}

	// We record 400 in the metrics to see how many events we are refusing, but the API
	// always returns 200 OK
	defer metrics.RecordHttpResponse(r.Method, "httpMSC4284Sign", http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(msc4284NoSignature)
}

func signEvent(server *Homeserver, event gomatrixserverlib.PDU, w http.ResponseWriter) {
	defer metrics.RecordHttpResponse("POST", "httpMSC4284Sign", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	event.Redact()
	signedEventJSON, err := gomatrixserverlib.SignJSON(
		string(server.ServerName), PolicyServerKeyID, server.eventSigningKey, event.JSON(),
	)
	if err != nil {
		log.Println("Error signing JSON:", err)
		_, _ = w.Write(msc4284NoSignature)
		return
	}

	// the API wants us to only return the signatures block e.g
	// { "$policy_server_via_domain" : { "ed25519:policy_server": "signature_base64" }}
	var sigs signatures
	if err = json.Unmarshal(signedEventJSON, &sigs); err != nil {
		log.Println("Error extracting signature from JSON:", err)
		_, _ = w.Write(msc4284NoSignature)
		return
	}
	sigBase64 := sigs.Signatures[string(server.ServerName)][PolicyServerKeyID]

	onlyPolicyServerSignature := signatures{
		Signatures: make(map[string]map[gomatrixserverlib.KeyID]spec.Base64Bytes),
	}
	onlyPolicyServerSignature.Signatures[string(server.ServerName)] = map[gomatrixserverlib.KeyID]spec.Base64Bytes{
		PolicyServerKeyID: sigBase64,
	}
	responseBody, err := json.Marshal(onlyPolicyServerSignature.Signatures)
	if err != nil {
		log.Println("Error creating signature response:", err)
		_, _ = w.Write(msc4284NoSignature)
		return
	}

	_, _ = w.Write(responseBody)
}
