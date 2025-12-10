package dbmetrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RoomMetadataCacheRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_room_metadata_cache_requests",
	Help: "The total number of room metadata cache requests",
}, []string{"isHit"})

var EventCacheRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_event_cache_requests",
	Help: "The total number of event cache requests",
}, []string{"isHit"})

func RecordRoomMetadataCacheRequest(isHit bool) {
	RoomMetadataCacheRequests.With(prometheus.Labels{
		"isHit": strconv.FormatBool(isHit),
	}).Inc()
}

func RecordEventCacheRequest(isHit bool) {
	EventCacheRequests.With(prometheus.Labels{
		"isHit": strconv.FormatBool(isHit),
	}).Inc()
}
