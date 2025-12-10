package metrics

import (
	"strconv"

	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var EventCheckRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_event_check_requests",
	Help: "The total number of event check requests",
}, []string{"roomId"})

var EventChecks = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_event_checks",
	Help: "The total number of actual event checks",
}, []string{"roomId", "status", "isFirstTime"})

var EventClassifications = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_event_classifications",
	Help: "The total number of classifications used by filters",
}, []string{"roomId", "classification"})

func RecordEventCheckRequest(roomId string) {
	EventCheckRequests.With(prometheus.Labels{
		"roomId": roomId,
	}).Inc()
}

func RecordFailedEventCheck(roomId string) {
	EventChecks.With(prometheus.Labels{
		"roomId":      roomId,
		"status":      "error",
		"isFirstTime": "false",
	}).Inc()
}

func RecordSuccessfulEventCheck(roomId string, isFirstTimeCheck bool, vecs confidence.Vectors) {
	EventChecks.With(prometheus.Labels{
		"roomId":      roomId,
		"status":      "ok",
		"isFirstTime": strconv.FormatBool(isFirstTimeCheck),
	}).Inc()
	if isFirstTimeCheck {
		// We're less concerned about the vector value and more about the types of classifications applied.
		for cls, _ := range vecs {
			EventClassifications.With(prometheus.Labels{
				"roomId":         roomId,
				"classification": cls.String(),
			}).Inc()
		}
	}
}
