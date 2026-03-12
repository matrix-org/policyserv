package homeserver

import (
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
)

func httpPolicySign(server *Homeserver, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpPolicySign")
	t := metrics.StartRequestTimer(r.Method, "httpPolicySign")
	defer t.ObserveDuration()

	handlePolicySignRequest(server, w, r, true)
}
