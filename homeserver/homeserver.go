package homeserver

import (
	"crypto/ed25519"
	"fmt"
	"log"
	"net/http"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

type KeyQueryServer struct {
	Name           string
	PreferredKeyId string
	PreferredKey   ed25519.PublicKey
}

type Config struct {
	ServerName        string
	PrivateSigningKey ed25519.PrivateKey
	// The key used to sign events as neutral. This key is not published or used as a server key.
	PrivateEventSigningKey  ed25519.PrivateKey
	SigningKeyVersion       string
	KeyQueryServer          *KeyQueryServer
	ActorLocalpart          string
	CacheRoomStateFor       time.Duration
	TrustedOrigins          []string
	EnableDirectKeyFetching bool
	MediaClientUrl          string
	MediaClientAccessToken  string
	AdminContacts           []*config.SupportContact
	SecurityContacts        []*config.SupportContact
	SupportUrl              string
}

type Homeserver struct {
	ServerName             spec.ServerName
	KeyId                  gomatrixserverlib.KeyID
	signingKey             ed25519.PrivateKey
	eventSigningKey        ed25519.PrivateKey
	client                 fclient.FederationClient
	localActor             spec.UserID
	storage                storage.PersistentStorage
	pubsubClient           pubsub.Client
	pool                   *queue.Pool
	keyCache               *cache.Cache[string, map[string]gomatrixserverlib.PublicKeyLookupResult] // server name -> {key ID -> result}
	keyRing                *gomatrixserverlib.KeyRing
	cacheRoomStateFor      time.Duration
	trustedOrigins         []string
	stateLearnCache        *cache.Cache[string, bool] // room ID -> literally anything because we don't really care about the value
	mediaClientUrl         string
	mediaClientAccessToken string
	adminContacts          []*config.SupportContact
	securityContacts       []*config.SupportContact
	supportUrl             string
}

func NewHomeserver(config *Config, storage storage.PersistentStorage, pool *queue.Pool, pubsubClient pubsub.Client) (*Homeserver, error) {
	serverName := spec.ServerName(config.ServerName)
	keyId := gomatrixserverlib.KeyID(fmt.Sprintf("ed25519:%s", config.SigningKeyVersion))
	client := fclient.NewFederationClient([]*fclient.SigningIdentity{{
		ServerName: serverName,
		KeyID:      keyId,
		PrivateKey: config.PrivateSigningKey,
	}})
	keyFetchers := []gomatrixserverlib.KeyFetcher{
		// Note: we don't fetch keys directly to minimize risk of untrusted network access. We could try to set
		// up GMSL's infrastructure for minimizing it, but there's risk in that too. Instead, we just send all
		// key requests through the config-trusted server.
		&gomatrixserverlib.PerspectiveKeyFetcher{
			PerspectiveServerName: spec.ServerName(config.KeyQueryServer.Name),
			PerspectiveServerKeys: map[gomatrixserverlib.KeyID]ed25519.PublicKey{
				gomatrixserverlib.KeyID(config.KeyQueryServer.PreferredKeyId): config.KeyQueryServer.PreferredKey,
			},
			Client: client,
		},
	}
	if config.EnableDirectKeyFetching {
		log.Println("Direct key fetching is enabled! This may reduce performance as remote servers may take a long time to respond.")
		pubKey := config.PrivateSigningKey.Public().(ed25519.PublicKey)
		keyFetchers = append([]gomatrixserverlib.KeyFetcher{
			&gomatrixserverlib.DirectKeyFetcher{
				IsLocalServerName: func(server spec.ServerName) bool {
					// Note: this is the same as `*Homeserver#isSelf`
					return server == spec.ServerName(config.ServerName)
				},
				LocalPublicKey: []byte(pubKey),
				Client:         client,
			},
		}, keyFetchers...)
	}
	hs := &Homeserver{
		ServerName:             serverName,
		KeyId:                  keyId,
		signingKey:             config.PrivateSigningKey,
		eventSigningKey:        config.PrivateEventSigningKey,
		client:                 client,
		localActor:             spec.NewUserIDOrPanic(fmt.Sprintf("@%s:%s", config.ActorLocalpart, config.ServerName), false),
		storage:                storage,
		pubsubClient:           pubsubClient,
		pool:                   pool,
		cacheRoomStateFor:      config.CacheRoomStateFor,
		trustedOrigins:         config.TrustedOrigins,
		mediaClientUrl:         config.MediaClientUrl,
		mediaClientAccessToken: config.MediaClientAccessToken,
		adminContacts:          config.AdminContacts,
		securityContacts:       config.SecurityContacts,
		supportUrl:             config.SupportUrl,
		keyCache: cache.New[string, map[string]gomatrixserverlib.PublicKeyLookupResult](
			cache.WithJanitorInterval[string, map[string]gomatrixserverlib.PublicKeyLookupResult](10 * time.Minute),
		),
		keyRing: &gomatrixserverlib.KeyRing{
			KeyFetchers: keyFetchers,
			KeyDatabase: nil, // set to self once created
		},
		stateLearnCache: cache.New[string, bool](
			// We cache entries for ~5 minutes, so clean up somewhat on time
			cache.WithJanitorInterval[string, bool](5 * time.Minute),
		),
	}
	hs.keyRing.KeyDatabase = hs // implemented by keyring.go
	return hs, nil
}

func (h *Homeserver) GetPublicEventSigningKey() ed25519.PublicKey {
	return h.eventSigningKey.Public().(ed25519.PublicKey)
}

func (h *Homeserver) isSelf(name spec.ServerName) bool {
	return name == h.ServerName
}

func (h *Homeserver) httpRequestHandler(upstream func(homeserver *Homeserver, w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream(h, w, r)
	})
}

func (h *Homeserver) BindTo(mux *http.ServeMux) error {
	mux.Handle("/.well-known/matrix/server", h.httpRequestHandler(httpDiscovery))
	mux.Handle("/.well-known/matrix/org.matrix.msc4284.policy_server", h.httpRequestHandler(httpKeyDiscovery))
	mux.Handle("/.well-known/matrix/support", h.httpRequestHandler(httpSupport))
	mux.Handle("/_matrix/federation/v1/version", h.httpRequestHandler(httpVersion))
	mux.Handle("/_matrix/key/v2/server", h.httpRequestHandler(httpSelfKey))
	mux.Handle("/_matrix/federation/v1/send/{txnId}", h.httpRequestHandler(httpTransactionReceive))
	mux.Handle("/_matrix/federation/v1/user/devices/{userId}", h.httpRequestHandler(httpUserDevices))
	mux.Handle("/_matrix/policy/unstable/org.matrix.msc4284/event/{eventId}/check", h.httpRequestHandler(httpMSC4284Check))
	mux.Handle("/_matrix/policy/unstable/org.matrix.msc4284/sign", h.httpRequestHandler(httpMSC4284Sign))
	return nil
}
