package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HttpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_http_requests",
	Help: "The total number of HTTP requests",
}, []string{"method", "action"})

var HttpResponses = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_http_responses",
}, []string{"method", "action", "status"})

func RecordHttpRequest(method string, action string) {
	HttpRequests.With(prometheus.Labels{
		"method": method,
		"action": action,
	}).Inc()
}

func RecordHttpResponse(method string, action string, status int) {
	HttpResponses.With(prometheus.Labels{
		"method": method,
		"action": action,
		"status": strconv.Itoa(status),
	}).Inc()
}
