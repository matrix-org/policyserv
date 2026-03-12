package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"strings"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

func setupHomeserver(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, pool *queue.Pool, pubsubClient pubsub.Client) (*homeserver.Homeserver, error) {
	// Capture signing keys
	key := decodeSigningKey(instanceConfig.HomeserverSigningKeyPath)
	eventKey := decodeSigningKey(instanceConfig.HomeserverEventSigningKeyPath)

	// Validate key query server
	if len(instanceConfig.KeyQueryServer) != 3 {
		log.Fatalf("KeyQueryServer must contain exactly 3 elements: serverName, keyId, base64Key")
	}
	if !strings.HasPrefix(instanceConfig.KeyQueryServer[1], "ed25519:") {
		log.Fatalf("KeyQueryServer must start with 'ed25519:' prefix")
	}
	b, err := base64.RawStdEncoding.DecodeString(instanceConfig.KeyQueryServer[2])
	if err != nil {
		log.Fatal(err) // configuration error
	}

	// Prepare a config
	hsConfig := &homeserver.Config{
		ServerName:              instanceConfig.HomeserverName,
		ActorLocalpart:          instanceConfig.JoinLocalpart,
		PrivateSigningKey:       key.PrivateKey,
		PrivateEventSigningKey:  eventKey.PrivateKey,
		SigningKeyVersion:       key.KeyVersion,
		CacheRoomStateFor:       time.Duration(instanceConfig.StateCacheMinutes) * time.Minute,
		TrustedOrigins:          instanceConfig.TrustedOrigins,
		EnableDirectKeyFetching: instanceConfig.EnableDirectKeyFetching,
		MediaClientUrl:          instanceConfig.HomeserverMediaClientUrl,
		MediaClientAccessToken:  instanceConfig.HomeserverMediaClientAccessToken,
		AdminContacts:           instanceConfig.SupportAdminContacts,
		SecurityContacts:        instanceConfig.SupportSecurityContacts,
		SupportUrl:              instanceConfig.SupportUrl,
		KeyQueryServer: &homeserver.KeyQueryServer{
			Name:           instanceConfig.KeyQueryServer[0],
			PreferredKeyId: instanceConfig.KeyQueryServer[1],
			PreferredKey:   ed25519.PublicKey(b),
		},
	}

	// Create the dependency and return
	return homeserver.NewHomeserver(hsConfig, storage, pool, pubsubClient)
}
