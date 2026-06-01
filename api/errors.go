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
	harms  []string
}

func (e *errorResponder) addHarm(harm string) *errorResponder {
	e.harms = append(e.harms, harm)
	return e
}

func (e *errorResponder) text(httpCode int, errcode string, error string) {
	defer metrics.RecordHttpResponse(e.r.Method, e.action, httpCode)
	homeserver.MustServeError(e.w, &homeserver.ClientError{
		HttpCode: httpCode,
		Errcode:  errcode,
		Message:  error,
		AdditionalFields: map[string]any{
			"org.matrix.msc4387.harms": e.harms,
		},
	})
}

func (e *errorResponder) err(httpCode int, errcode string, err error) {
	log.Printf("%s error (%d/%s): %v", e.action, httpCode, errcode, err)
	homeserver.MustServeError(e.w, &homeserver.ClientError{
		HttpCode: httpCode,
		Errcode:  errcode,
		Message:  "Error",
		AdditionalFields: map[string]any{
			"org.matrix.msc4387.harms": e.harms,
		},
	})
}

func newErrorResponder(action string, w http.ResponseWriter, r *http.Request) *errorResponder {
	return &errorResponder{
		action: action,
		w:      w,
		r:      r,
		harms:  make([]string, 0),
	}
}
