package api

import (
	"net/http"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/metrics"
)

func httpGetInstanceConfigApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpGetInstanceConfigApi")
	t := metrics.StartRequestTimer(r.Method, "httpGetInstanceConfigApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpGetInstanceConfigApi", w, r)

	if r.Method != http.MethodGet {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	instanceConfig, err := config.NewCommunityConfigForJSON(nil)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpGetInstanceConfigApi", r, w, instanceConfig)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
