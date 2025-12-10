package api

import (
	"log"
	"net/http"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/metrics"
)

type errorResponder struct {
	action string
	w      http.ResponseWriter
	r      *http.Request
}

func (e *errorResponder) text(httpCode int, errcode string, error string) {
	defer metrics.RecordHttpResponse(e.r.Method, e.action, httpCode)
	homeserver.MatrixHttpError(e.w, httpCode, errcode, error)
}

func (e *errorResponder) err(httpCode int, errcode string, err error) {
	log.Printf("%s error (%d/%s): %v", e.action, httpCode, errcode, err)
	e.text(httpCode, errcode, "Error")
}

func newErrorResponder(action string, w http.ResponseWriter, r *http.Request) *errorResponder {
	return &errorResponder{
		action: action,
		w:      w,
		r:      r,
	}
}
