package test

import (
	"github.com/matrix-org/policyserv/filter/audit"
)

func MustMakeAuditQueue(size int) *audit.Queue {
	queue, err := audit.NewQueue(size)
	if err != nil {
		panic(err)
	}
	return queue
}
