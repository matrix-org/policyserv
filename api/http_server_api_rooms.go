package api

import (
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
)

func httpJoinRoomCommunityApi(api *Api, community *storage.StoredCommunity, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpJoinRoomCommunityApi")
	t := metrics.StartRequestTimer(r.Method, "httpJoinRoomCommunityApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpJoinRoomCommunityApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	if !community.CanSelfJoinRooms {
		errs.text(http.StatusForbidden, "M_FORBIDDEN", "This community cannot self-serve add rooms")
		return
	}

	roomId := r.PathValue("roomId")
	doHttpAddRoom("httpJoinRoomCommunityApi", api, w, r, roomId, community)
}
