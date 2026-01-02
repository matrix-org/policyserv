package api

import (
	"database/sql"
	"errors"
	"io"
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
)

func httpKeywordTemplates(api *Api, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		getKeywordTemplateHandler(api, w, r)
	} else if r.Method == http.MethodPost {
		setKeywordTemplateHandler(api, w, r)
	} else {
		errs := newErrorResponder("httpKeywordTemplates", w, r)
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
	}
}

func setKeywordTemplateHandler(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "setKeywordTemplateHandler")
	t := metrics.StartRequestTimer(r.Method, "setKeywordTemplateHandler")
	defer t.ObserveDuration()

	errs := newErrorResponder("setKeywordTemplateHandler", w, r)

	name := r.PathValue("name")
	b, err := io.ReadAll(r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_UNKNOWN", err)
		return
	}
	body := string(b)

	val := &storage.StoredKeywordTemplate{
		Name: name,
		Body: body,
	}
	err = api.storage.UpsertKeywordTemplate(r.Context(), val)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpSetKeywordTemplateApi", r, w, val)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}

func getKeywordTemplateHandler(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "getKeywordTemplateHandler")
	t := metrics.StartRequestTimer(r.Method, "getKeywordTemplateHandler")
	defer t.ObserveDuration()

	errs := newErrorResponder("getKeywordTemplateHandler", w, r)

	name := r.PathValue("name")
	val, err := api.storage.GetKeywordTemplate(r.Context(), name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errs.text(http.StatusNotFound, "M_NOT_FOUND", "Template not found")
		} else {
			errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		}
		return
	}

	err = respondJson("httpGetKeywordTemplateApi", r, w, val)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
