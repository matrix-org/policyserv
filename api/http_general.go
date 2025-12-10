package api

import (
	"log"
	"net/http"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/metrics"
)

func httpHealth(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpHealth")
	t := metrics.StartRequestTimer(r.Method, "httpHealth")
	defer t.ObserveDuration()

	defer metrics.RecordHttpResponse(r.Method, "httpHealth", http.StatusOK)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func httpReady(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpReady")
	t := metrics.StartRequestTimer(r.Method, "httpReady")
	defer t.ObserveDuration()

	defer metrics.RecordHttpResponse(r.Method, "httpReady", http.StatusOK)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func httpCatchAll(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpCatchAll")
	t := metrics.StartRequestTimer(r.Method, "httpCatchAll")
	defer t.ObserveDuration()

	// To appease blackbox exporters
	if r.URL.Path == "/" {
		_, _ = w.Write([]byte("ok"))
		defer metrics.RecordHttpResponse(r.Method, "httpCatchAll", http.StatusOK)
		return
	}

	log.Printf("Unhandled request: %s, %s\n", r.Method, r.URL)

	defer metrics.RecordHttpResponse(r.Method, "httpCatchAll", http.StatusNotFound)
	homeserver.MatrixHttpError(w, http.StatusNotFound, "M_UNRECOGNIZED", "not implemented")
}
