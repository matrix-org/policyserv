package main

import (
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

func setupQueue(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, communityManager *community.Manager) (*queue.Pool, error) {
	poolConfig := &queue.PoolConfig{
		ConcurrentPools: 10,
		SizePerPool:     instanceConfig.ProcessingPoolSize / 10,
	}
	return queue.NewPool(poolConfig, communityManager, storage)
}
