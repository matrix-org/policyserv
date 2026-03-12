package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HMAHashTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "policyserv_hma_hash_time_seconds",
	Help: "The time spent in the HMA hash call",
}, []string{"contentType"})

var HMAMatchTime = promauto.NewHistogram(prometheus.HistogramOpts{
	Name: "policyserv_hma_match_time_seconds",
	Help: "The time spent in the HMA match call",
})

var HMAHashRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_hma_hash_requests",
	Help: "The total number of HMA hash requests",
}, []string{"contentType"})

var HMAMatchRequests = promauto.NewCounter(prometheus.CounterOpts{
	Name: "policyserv_hma_match_requests",
	Help: "The total number of HMA match requests",
})

var HMABankHits = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_hma_bank_hits",
	Help: "The total number of HMA bank hits",
}, []string{"contentType", "bankName", "enabled"})

func StartHMAHashTimer(contentType string) *prometheus.Timer {
	return prometheus.NewTimer(HMAHashTime.With(prometheus.Labels{
		"contentType": contentType,
	}))
}

func StartHMAMatchTimer() *prometheus.Timer {
	return prometheus.NewTimer(HMAMatchTime)
}

func RecordHMAHashRequest(contentType string) {
	HMAHashRequests.With(prometheus.Labels{
		"contentType": contentType,
	}).Inc()
}

func RecordHMAMatchRequest() {
	HMAMatchRequests.Inc()
}

func RecordHMABankHit(contentType string, bankName string, enabled bool) {
	HMABankHits.With(prometheus.Labels{
		"contentType": contentType,
		"bankName":    bankName,
		"enabled":     strconv.FormatBool(enabled),
	}).Inc()
}
