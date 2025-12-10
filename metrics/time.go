package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var FilterTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_filter_time_seconds",
	Help: "The time spent in each filter",
}, []string{"roomId", "filterName"})

var RequestTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_request_time_seconds",
	Help: "The time spent in each request",
}, []string{"method", "action"})

var QueueWaitTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_queue_wait_time_seconds",
	Help: "The time spent waiting in the queue",
}, []string{"waitedUntil"})

var ModerationActionTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_moderation_action_time_seconds",
	Help: "The time spent in each moderation action",
}, []string{"action"})

func StartFilterTimer(roomId string, filterName string) *prometheus.Timer {
	return prometheus.NewTimer(FilterTime.With(prometheus.Labels{
		"roomId":     roomId,
		"filterName": filterName,
	}))
}

func StartRequestTimer(method string, action string) *prometheus.Timer {
	return prometheus.NewTimer(RequestTime.With(prometheus.Labels{
		"method": method,
		"action": action,
	}))
}

func StartQueueTimer() *prometheus.Timer {
	return prometheus.NewTimer(QueueWaitTime.With(prometheus.Labels{
		"waitedUntil": "UNSET",
	}))
}

func StartModerationActionTimer(action ModerationAction) *prometheus.Timer {
	return prometheus.NewTimer(ModerationActionTime.With(prometheus.Labels{
		"action": string(action),
	}))
}
