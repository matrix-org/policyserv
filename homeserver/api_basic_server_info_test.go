package homeserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestHttpKeyDiscovery(t *testing.T) {
	t.Parallel()

	// Because we're interacting with very little of the interface, we can create a Homeserver instance without using
	// the NewHomeserver() function.
	_, eventSigningKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	server := &Homeserver{
		eventSigningKey: eventSigningKey,
	}

	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)

		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/org.matrix.msc4284.policy_server", nil)

		httpKeyDiscovery(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(eventSigningKey.Public().(ed25519.PublicKey))
		test.AssertJsonBody(t, res, map[string]any{"public_key": b64})
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
		assert.Equal(t, "*", res.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", res.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "X-Requested-With, Content-Type, Authorization", res.Header().Get("Access-Control-Allow-Headers"))
	}

	// Test wrong method too while we're here
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/.well-known/matrix/org.matrix.msc4284.policy_server", nil)
	httpKeyDiscovery(server, res, req)
	assert.Equal(t, http.StatusMethodNotAllowed, res.Code)
	test.AssertApiError(t, res, "M_UNRECOGNIZED", "Method not allowed")
}

func TestHttpSupport(t *testing.T) {
	t.Parallel()

	// Because we're interacting with very little of the interface, we can create a Homeserver instance without using
	// the NewHomeserver() function.
	server := &Homeserver{
		adminContacts: []*config.SupportContact{
			{Value: "@admin:example.org", Type: config.SupportContactTypeMatrixUserId},
			{Value: "admin@example.org", Type: config.SupportContactTypeEmail},
			{Value: "skipped@example.org", Type: "UNKNOWN"},
		},
		securityContacts: []*config.SupportContact{
			{Value: "@security:example.org", Type: config.SupportContactTypeMatrixUserId},
			{Value: "security@example.org", Type: config.SupportContactTypeEmail},
			{Value: "skipped@example.org", Type: "UNKNOWN"},
		},
		supportUrl: "https://example.org/support",
	}

	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/server", nil)
		httpSupport(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		test.AssertJsonBody(t, res, map[string]any{
			"support_page": server.supportUrl,
			"contacts": []map[string]any{
				{"matrix_id": "@admin:example.org", "role": "m.role.admin"},
				{"email_address": "admin@example.org", "role": "m.role.admin"},
				{"matrix_id": "@security:example.org", "role": "m.role.security"},
				{"email_address": "security@example.org", "role": "m.role.security"},
				// Note: "UNKNOWN" contacts are not included because they're unknown
			},
		})
	}

	// Now start removing things from the support object to ensure it still generates a response

	server.adminContacts = make([]*config.SupportContact, 0)
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/server", nil)
		httpSupport(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		test.AssertJsonBody(t, res, map[string]any{
			"support_page": server.supportUrl,
			"contacts": []map[string]any{
				{"matrix_id": "@security:example.org", "role": "m.role.security"},
				{"email_address": "security@example.org", "role": "m.role.security"},
				// Note: "UNKNOWN" contacts are not included because they're unknown
			},
		})
	}

	server.securityContacts = make([]*config.SupportContact, 0)
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/server", nil)
		httpSupport(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		test.AssertJsonBody(t, res, map[string]any{
			"support_page": server.supportUrl,
			// There are no contacts left, so the response shouldn't have any
		})
	}

	// If we also remove the support URL, we should get a 404 instead
	server.supportUrl = ""
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/server", nil)
		httpSupport(server, res, req)
		assert.Equal(t, http.StatusNotFound, res.Code)
		test.AssertApiError(t, res, "M_NOT_FOUND", "No support information available")
	}

	// Then, if we add back some contacts, we should get a useful response again
	server.adminContacts = []*config.SupportContact{
		{Value: "admin@example.org", Type: config.SupportContactTypeEmail},
	}
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/server", nil)
		httpSupport(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		test.AssertJsonBody(t, res, map[string]any{
			// This time there should be contacts without a support_page
			"contacts": []map[string]any{
				{"email_address": "admin@example.org", "role": "m.role.admin"},
			},
		})
	}
}
