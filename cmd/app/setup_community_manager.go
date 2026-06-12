package main

import (
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/notifiers"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

func setupCommunityManager(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, pubsub pubsub.Client, notifier notifiers.MatrixNotifier) (*community.Manager, error) {
	return community.NewManager(instanceConfig, storage, pubsub, notifier)
}
