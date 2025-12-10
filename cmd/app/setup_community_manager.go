package main

import (
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/audit"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

func setupCommunityManager(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, pubsub pubsub.Client, auditQueue *audit.Queue) (*community.Manager, error) {
	return community.NewManager(instanceConfig, storage, pubsub, auditQueue)
}
