package storage

import (
	"github.com/segmentio/ksuid"
)

func NextId() string {
	// There's technically a chance of collisions with ksuid, but as it helpfully explains, it's
	// infeasible to do so within the current limitations of physics.
	return ksuid.New().String()
}
