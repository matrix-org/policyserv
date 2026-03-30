package api

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
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

func httpCommunities(api *Api, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		communityGetHandler(api, w, r)
	} else if r.Method == http.MethodPatch {
		communityPatchHandler(api, w, r)
	} else {
		errs := newErrorResponder("httpCommunities", w, r)
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
	}
}

func communityGetHandler(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "communityGetHandler")
	t := metrics.StartRequestTimer(r.Method, "communityGetHandler")
	defer t.ObserveDuration()

	errs := newErrorResponder("communityGetHandler", w, r)

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

	err = respondJson("communityGetHandler", r, w, community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}

func communityPatchHandler(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "communityPatchHandler")
	t := metrics.StartRequestTimer(r.Method, "communityPatchHandler")
	defer t.ObserveDuration()

	errs := newErrorResponder("communityPatchHandler", w, r)

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

	// Pull out some variables we don't want to change via this endpoint
	communityId := community.CommunityId
	accessToken := community.ApiAccessToken

	// Apply the request body over top of the community object
	err = parseJsonBody(&community, r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}

	// Reset the unchangeable variables (we could also detect changes and error, but this works too)
	community.CommunityId = communityId
	community.ApiAccessToken = accessToken

	// Update in the database before returning
	err = api.storage.UpsertCommunity(r.Context(), community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("communityGetHandler", r, w, community)
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

func httpRotateCommunityAccessTokenApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpRotateCommunityAccessTokenApi")
	t := metrics.StartRequestTimer(r.Method, "httpRotateCommunityAccessTokenApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpRotateCommunityAccessTokenApi", w, r)

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

	oldAccessToken := internal.Dereference(community.ApiAccessToken)

	newAccessToken := fmt.Sprintf("pst_%s", rand.Text())
	community.ApiAccessToken = internal.Pointer(newAccessToken)
	err = api.storage.UpsertCommunity(r.Context(), community)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	err = respondJson("httpRotateCommunityAccessTokenApi", r, w, map[string]string{
		"old_access_token": oldAccessToken,
		"new_access_token": newAccessToken,
	})
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
