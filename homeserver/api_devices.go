package homeserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

func httpUserDevices(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpUserDevices")
	t := metrics.StartRequestTimer(r.Method, "httpUserDevices")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet {
		defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return
	}

	_, fedErr := fclient.VerifyHTTPRequest(r, time.Now(), server.ServerName, server.isSelf, server.keyRing)
	if !fedErr.Is2xx() {
		b, err := json.Marshal(fedErr.JSON)
		if err != nil {
			log.Println("Error marshalling fedErr:", err)
			defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusInternalServerError)
			MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal error response")
			return
		}

		defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", fedErr.Code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fedErr.Code)
		_, _ = w.Write(b)
		return
	}

	userId, err := spec.NewUserID(r.PathValue("userId"), true)
	if err != nil {
		log.Println("Error parsing requested user ID:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to parse user ID")
		return
	}

	if userId.Domain() != server.ServerName {
		defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusNotFound)
		MatrixHttpError(w, http.StatusNotFound, "M_NOT_FOUND", "Unknown user ID")
		return
	}

	// We don't actually track devices on our users, so just respond with something to stop the requesting server
	// asking about it.
	resp := fclient.RespUserDevices{
		UserID:         userId.String(),
		StreamID:       1,
		Devices:        make([]fclient.RespUserDevice, 0),
		MasterKey:      nil,
		SelfSigningKey: nil,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal response")
		return
	}

	defer metrics.RecordHttpResponse(r.Method, "httpUserDevices", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
