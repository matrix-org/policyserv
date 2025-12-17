package homeserver

import (
	"context"
	"log"
	"net/http"

	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

// Ping - Sends an authenticated request to the given server name in an attempt to encourage
// it to send us transactions/requests.
func (h *Homeserver) Ping(ctx context.Context, serverName string) error {
	fedReq := fclient.NewFederationRequest(http.MethodGet, h.ServerName, spec.ServerName(serverName), "/_matrix/federation/v1/version")
	err := fedReq.Sign(h.ServerName, h.KeyId, h.signingKey)
	if err != nil {
		return err
	}
	req, err := fedReq.HTTPRequest()
	if err != nil {
		return err
	}

	res := &fclient.Version{}
	err = h.client.DoRequestAndParseResponse(ctx, req, &res)
	if err != nil {
		return err
	}

	log.Printf("Ping: serverName=%s version=%+v", serverName, res.Server)

	return nil
}
