package homeserver

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/version"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

func httpDiscovery(srv *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpDiscovery")
	t := metrics.StartRequestTimer(r.Method, "httpDiscovery")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet {
		defer metrics.RecordHttpResponse(r.Method, "httpDiscovery", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return
	}

	defer metrics.RecordHttpResponse(r.Method, "httpDiscovery", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"m.server":"%s:443"}`, srv.ServerName)))
}

func httpVersion(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpVersion")
	t := metrics.StartRequestTimer(r.Method, "httpVersion")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet {
		defer metrics.RecordHttpResponse(r.Method, "httpVersion", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return
	}

	defer metrics.RecordHttpResponse(r.Method, "httpVersion", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"server":{"name":"policyserv","version":"%s"}}`, version.Revision)))
	return
}

func httpSelfKey(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpSelfKey")
	t := metrics.StartRequestTimer(r.Method, "httpSelfKey")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet {
		defer metrics.RecordHttpResponse(r.Method, "httpSelfKey", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNKNOWN", "Method not allowed")
		return
	}

	keys := gomatrixserverlib.ServerKeys{}
	keys.ServerName = server.ServerName
	keys.ValidUntilTS = spec.AsTimestamp(time.Now().Add(signingKeyExpiration))
	keys.VerifyKeys = map[gomatrixserverlib.KeyID]gomatrixserverlib.VerifyKey{
		server.KeyId: {
			Key: spec.Base64Bytes(server.signingKey.Public().(ed25519.PublicKey)),
		},
	}
	keys.OldVerifyKeys = make(map[gomatrixserverlib.KeyID]gomatrixserverlib.OldVerifyKey) // not populated

	toSign, err := json.Marshal(keys.ServerKeyFields)
	if err != nil {
		log.Println("Error marshalling keys:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpSelfKey", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal key fields")
		return
	}
	keys.Raw, err = gomatrixserverlib.SignJSON(string(server.ServerName), server.KeyId, server.signingKey, toSign)
	if err != nil {
		log.Println("Error signing key fields:", err)
		defer metrics.RecordHttpResponse(r.Method, "httpSelfKey", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to sign key fields")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(keys.Raw)
}
