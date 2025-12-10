package api

import (
	"net/http"
	"strings"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/metrics"
)

func httpCreateCommunityApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpCreateCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpCreateCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpCreateCommunityApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	req := struct {
		Name string `json:"name"`
	}{}
	err := parseJsonBody(&req, r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		errs.text(http.StatusBadRequest, "M_BAD_JSON", "Name is required")
		return
	}
	if len(req.Name) <= 3 || len(req.Name) >= 255 {
		errs.text(http.StatusBadRequest, "M_BAD_JSON", "Name must be between 3 and 255 characters")
		return
	}

	community, err := api.storage.CreateCommunity(r.Context(), req.Name)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpCreateCommunityApi", r, w, community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}

func httpGetCommunityApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpGetCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpGetCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpGetCommunityApi", w, r)

	if r.Method != http.MethodGet {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	id := r.PathValue("id")
	community, err := api.storage.GetCommunity(r.Context(), id)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
	if community == nil {
		errs.text(http.StatusNotFound, "M_NOT_FOUND", "Community not found")
		return
	}

	err = respondJson("httpGetCommunityApi", r, w, community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}

func httpSetCommunityConfigApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpSetCommunityConfigApi")
	t := metrics.StartRequestTimer(r.Method, "httpSetCommunityConfigApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpSetCommunityConfigApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	id := r.PathValue("id")
	community, err := api.storage.GetCommunity(r.Context(), id)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
	if community == nil {
		errs.text(http.StatusNotFound, "M_NOT_FOUND", "Community not found")
		return
	}

	req := &config.CommunityConfig{}
	err = parseJsonBody(req, r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}

	community.Config = req
	err = api.storage.UpsertCommunity(r.Context(), community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpSetCommunityConfigApi", r, w, community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
