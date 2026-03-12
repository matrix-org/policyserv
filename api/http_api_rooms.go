package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/metrics"
)

func httpGetRoomsApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpGetRoomsApi")
	t := metrics.StartRequestTimer(r.Method, "httpGetRoomsApi")
	defer t.ObserveDuration()

	if r.Method != http.MethodGet {
		defer metrics.RecordHttpResponse(r.Method, "httpGetRoomsApi", http.StatusMethodNotAllowed)
		homeserver.MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	rooms, err := api.storage.GetAllRooms(r.Context())
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpGetRoomsApi", http.StatusInternalServerError)
		homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Error")
		return
	}

	b, err := json.Marshal(rooms)
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpGetRoomsApi", http.StatusInternalServerError)
		homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Error")
		return
	}

	defer metrics.RecordHttpResponse(r.Method, "httpGetRoomsApi", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func httpGetRoomApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpGetRoomApi")
	t := metrics.StartRequestTimer(r.Method, "httpGetRoomApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpGetRoomApi", w, r)

	if r.Method != http.MethodGet {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	id := r.PathValue("id")
	room, err := api.storage.GetRoom(r.Context(), id)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
	if room == nil {
		errs.text(http.StatusNotFound, "M_NOT_FOUND", "Room not found")
		return
	}

	err = respondJson("httpGetRoomApi", r, w, room)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}

func httpAddRoomApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpAddRoomApi")
	t := metrics.StartRequestTimer(r.Method, "httpAddRoomApi")
	defer t.ObserveDuration()

	errs := newErrorResponder("httpAddRoomApi", w, r)

	if r.Method != http.MethodPost {
		errs.text(http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	req := struct {
		CommunityId string `json:"community_id"`
	}{}
	err := parseJsonBody(&req, r.Body)
	if err != nil {
		errs.err(http.StatusBadRequest, "M_BAD_JSON", err)
		return
	}

	// Ensure the community exists
	community, err := api.storage.GetCommunity(r.Context(), req.CommunityId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
	if community == nil {
		errs.text(http.StatusBadRequest, "M_BAD_STATE", "Community not found")
		return
	}

	// Ensure the room *doesn't* exist
	roomId := r.PathValue("roomId")
	room, err := api.storage.GetRoom(r.Context(), roomId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
	}
	if room != nil {
		errs.text(http.StatusBadRequest, "M_BAD_STATE", "Room already exists")
		return
	}

	// Try to join the new room (this will add it to the database)
	room, err = api.hs.JoinRoom(r.Context(), roomId, api.joinViaServer, community.CommunityId)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}

	// Respond with the room's config details
	err = respondJson("httpAddRoomApi", r, w, room)
	if err != nil {
		errs.err(http.StatusInternalServerError, "M_UNKNOWN", err)
		return
	}
}
