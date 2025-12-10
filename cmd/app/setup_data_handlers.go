package main

import (
	"errors"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

func setupDataHandlers(instanceConfig *config.InstanceConfig) (storage.PersistentStorage, pubsub.Client, error) {
	dbConfig := &storage.PostgresStorageConfig{
		RWDatabase: &storage.PostgresStorageConnectionConfig{
			Uri:          instanceConfig.Database,
			MaxOpenConns: instanceConfig.DatabaseMaxOpenConns,
			MaxIdleConns: instanceConfig.DatabaseMaxIdleConns,
		},
		RODatabase: &storage.PostgresStorageConnectionConfig{
			Uri:          instanceConfig.DatabaseReadonlyUri,
			MaxOpenConns: instanceConfig.DatabaseReadonlyMaxOpen,
			MaxIdleConns: instanceConfig.DatabaseReadonlyMaxIdle,
		},
		MigrationsPath: instanceConfig.DatabaseMigrationsDir,
	}
	psqlDb, err := storage.NewPostgresStorage(dbConfig)
	if err != nil {
		return nil, nil, errors.Join(errors.New("NewPostgresStorage: failed create"), err)
	}
	pubsubConfig := &pubsub.PostgresPubsubConnectionConfig{
		Uri:                  instanceConfig.Database,
		MinReconnectInterval: 100 * time.Millisecond,
		MaxReconnectInterval: 5 * time.Second,
	}
	psqlPubsub, err := pubsub.NewPostgresPubsub(psqlDb, pubsubConfig)
	if err != nil {
		return nil, nil, errors.Join(errors.New("NewPostgresPubsub: failed create"), err)
	}
	return psqlDb, psqlPubsub, nil
}
