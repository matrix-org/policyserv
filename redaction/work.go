package redaction

import (
	"log"

	"github.com/matrix-org/policyserv/metrics"
)

func doWork(roomId string, eventId string, client *Client) {
	t := metrics.StartModerationActionTimer(metrics.ModerationActionRedaction)
	defer t.ObserveDuration()

	log.Println("Redacting event", eventId, "in room", roomId)
	err := client.RedactEvent(roomId, eventId)
	if err != nil {
		log.Println("Error redacting event", eventId, "in room", roomId, ": ", err)
		metrics.RecordModerationAction(metrics.ModerationActionRedaction, metrics.ModerationStatusError)
	} else {
		log.Println("Successfully redacted event", eventId, "in room", roomId)
		metrics.RecordModerationAction(metrics.ModerationActionRedaction, metrics.ModerationStatusOk)
	}
}
