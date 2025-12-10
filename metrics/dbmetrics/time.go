package dbmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var SelfDatabaseRequestTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_self_database_request_time_seconds",
	Help: "The time spent in the self database",
}, []string{"query"})

func StartSelfDatabaseTimer(query string) *prometheus.Timer {
	return prometheus.NewTimer(SelfDatabaseRequestTime.With(prometheus.Labels{
		"query": query,
	}))
}
