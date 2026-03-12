package main

import (
	"github.com/matrix-org/policyserv/api"
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/storage"
)

func setupApi(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, hs *homeserver.Homeserver, communityManager *community.Manager) (*api.Api, error) {
	apiConfig := &api.Config{
		ApiKey:        instanceConfig.ApiKey,
		JoinViaServer: instanceConfig.JoinServer,
	}
	return api.NewApi(apiConfig, storage, hs, communityManager)
}
