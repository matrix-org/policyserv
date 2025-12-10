package api

import (
	"encoding/json"
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/trust"
)

func httpSetMuninnSourceData(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpSetMuninnSourceData")
	t := metrics.StartRequestTimer(r.Method, "httpSetMuninnSourceData")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpSetMuninnSourceData", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	val := &trust.MuninnMemberDirectoryEvent{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(val)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}

	source, err := trust.NewMuninnHallSource(api.storage)
	if err != nil {
		// "should never happen"
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = source.ImportData(r.Context(), val.Content.MemberDirectory)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpSetMuninnSourceDataApi", r, w, val)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
