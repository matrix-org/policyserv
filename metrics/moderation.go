package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ModerationAction string

const ModerationActionRedaction ModerationAction = "redaction"

type ModerationStatus string

const ModerationStatusOk ModerationStatus = "ok"
const ModerationStatusError ModerationStatus = "error"
const ModerationStatusNoModerator ModerationStatus = "no_moderator"
const ModerationStatusModeratorNotConfigured ModerationStatus = "moderator_not_configured"
const ModerationStatusOutOfBandModeration ModerationStatus = "out_of_band_moderation"

var ModerationRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_moderation_requests",
	Help: "The total number of moderation requests",
}, []string{"action"})

var ModerationActions = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "policyserv_moderation_actions",
	Help: "The total number of moderation actions",
}, []string{"action", "status"})

func RecordModerationRequest(action ModerationAction) {
	ModerationRequests.With(prometheus.Labels{
		"action": string(action),
	}).Inc()
}

func RecordModerationAction(action ModerationAction, status ModerationStatus) {
	ModerationActions.With(prometheus.Labels{
		"action": string(action),
		"status": string(status),
	}).Inc()
}
