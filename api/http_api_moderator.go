package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/metrics"
)

type setModeratorRequest struct {
	UserId string `json:"moderator_user_id"`
	RoomId string `json:"room_id"`
}

func httpSetModeratorApi(api *Api, w http.ResponseWriter, r *http.Request) {
	metrics.RecordHttpRequest(r.Method, "httpSetModeratorApi")
	t := metrics.StartRequestTimer(r.Method, "httpSetModeratorApi")
	defer t.ObserveDuration()

	if r.Method != http.MethodPost {
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusMethodNotAllowed)
		homeserver.MatrixHttpError(w, http.StatusMethodNotAllowed, "M_UNRECOGNIZED", "Method not allowed")
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusBadRequest)
		homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Error")
		return
	}

	req := setModeratorRequest{}
	err = json.Unmarshal(b, &req)
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusBadRequest)
		homeserver.MatrixHttpError(w, http.StatusBadRequest, "M_BAD_JSON", "Bad JSON")
		return
	}

	room, err := api.storage.GetRoom(r.Context(), req.RoomId)
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusInternalServerError)
		homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Error")
		return
	}
	if room == nil {
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusNotFound)
		homeserver.MatrixHttpError(w, http.StatusNotFound, "M_NOT_FOUND", "Room not found")
		return
	}
	room.ModeratorUserId = req.UserId
	err = api.storage.UpsertRoom(r.Context(), room)
	if err != nil {
		log.Println(err)
		defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusInternalServerError)
		homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Error")
		return
	}

	log.Println("Set moderator '", req.UserId, "' for room", req.RoomId)

	defer metrics.RecordHttpResponse(r.Method, "httpSetModeratorApi", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
