package homeserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/version"
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

func httpKeyDiscovery(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpKeyDiscovery")
	t := metrics.StartRequestTimer(r.Method, "httpKeyDiscovery")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet && r.Method != http.MethodOptions { // OPTIONS is for CORS support
		defer metrics.RecordHttpResponse(r.Method, "httpKeyDiscovery", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	// Set CORS headers per https://spec.matrix.org/v1.17/client-server-api/#web-browser-clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Authorization")

	defer metrics.RecordHttpResponse(r.Method, "httpKeyDiscovery", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(server.GetPublicEventSigningKey())
	_, _ = w.Write([]byte(fmt.Sprintf(`{"public_key":"%s"}`, b64)))
}

type wellknownSupportContact struct {
	// Per https://spec.matrix.org/v1.17/client-server-api/#getwell-knownmatrixsupport

	Email  string `json:"email_address,omitempty"`
	UserId string `json:"matrix_id,omitempty"`
	Role   string `json:"role"`
}

type wellknownSupport struct {
	// Per https://spec.matrix.org/v1.17/client-server-api/#getwell-knownmatrixsupport

	Contacts   []wellknownSupportContact `json:"contacts,omitempty"`
	SupportUrl string                    `json:"support_page,omitempty"`
}

func httpSupport(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpSupport")
	t := metrics.StartRequestTimer(r.Method, "httpSupport")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet && r.Method != http.MethodOptions { // OPTIONS is for CORS support
		defer metrics.RecordHttpResponse(r.Method, "httpSupport", http.StatusMethodNotAllowed)
		MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	// Set CORS headers per https://spec.matrix.org/v1.17/client-server-api/#web-browser-clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Authorization")

	if server.supportUrl == "" && len(server.adminContacts) == 0 && len(server.securityContacts) == 0 {
		// No support means 404 per spec
		defer metrics.RecordHttpResponse(r.Method, "httpSupport", http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		MatrixHttpError(w, http.StatusNotFound, "M_NOT_FOUND", "No support information available")
		return
	}

	val := wellknownSupport{
		Contacts:   make([]wellknownSupportContact, 0),
		SupportUrl: server.supportUrl,
	}
	appendContact := func(role string, contact config.SupportContact) {
		wkContact := wellknownSupportContact{
			Role: role,
		}
		if contact.Type == config.SupportContactTypeEmail {
			wkContact.Email = contact.Value
		} else if contact.Type == config.SupportContactTypeMatrixUserId {
			wkContact.UserId = contact.Value
		} else {
			return // skip, but "should never happen"
		}
		val.Contacts = append(val.Contacts, wkContact)
	}
	for _, c := range server.adminContacts {
		appendContact("m.role.admin", c)
	}
	for _, c := range server.securityContacts {
		appendContact("m.role.security", c)
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(val)
	if err != nil {
		defer metrics.RecordHttpResponse(r.Method, "httpSupport", http.StatusInternalServerError)
		MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Unable to marshal response")
		return
	}
	defer metrics.RecordHttpResponse(r.Method, "httpSupport", http.StatusOK)
	_, _ = w.Write(b)
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
