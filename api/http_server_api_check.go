package api

import (
	"io"
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
)

func httpCheckTextCommunityApi(api *Api, community *storage.StoredCommunity, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpCheckTextCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpCheckTextCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpCheckTextCommunityApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	set, err := api.communityManager.GetFilterSetForCommunityId(r.Context(), community.CommunityId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}
	textToCheck := string(b)

	vecs, err := set.CheckText(r.Context(), textToCheck)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	if set.IsSpamResponse(r.Context(), vecs) {
		// TODO: Also disclose MSC4387 harms, somehow
		errs.text(http.StatusBadRequest, "ORG.MATRIX.MSC4387_SAFETY", "Text is probably spammy")
	} else {
		err = respondJson("httpCheckTextCommunityApi", r, w, make(map[string]any))
		if err != nil {
			errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		}
	}
}
