package homeserver

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/matrix-org/gomatrixserverlib/fclient"
)

// Ping - Sends an authenticated request to the given server name in an attempt to encourage
// it to send us transactions/requests.
func (h *Homeserver) Ping(ctx context.Context, serverName string) error {
	mxUrl := url.URL{
		Scheme: "matrix",
		Host:   serverName,
		Path:   "/_matrix/federation/v1/version",
	}
	req, err := http.NewRequest(http.MethodGet, mxUrl.String(), nil)
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
