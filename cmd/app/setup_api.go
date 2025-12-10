package main

import (
	"github.com/matrix-org/policyserv/api"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/storage"
)

func setupApi(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, hs *homeserver.Homeserver) (*api.Api, error) {
	apiConfig := &api.Config{
		ApiKey:        instanceConfig.ApiKey,
		JoinViaServer: instanceConfig.JoinServer,
	}
	return api.NewApi(apiConfig, storage, hs)
}
