package homeserver

import (
	"context"
	"crypto/ed25519"
	"log"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

// How long until our signing keys expire, from time of request
const signingKeyExpiration = 24 * time.Hour

// How long we want to keep signing keys around in our cache
const cachedSigningKeyDuration = 1 * time.Hour

func (h *Homeserver) FetcherName() string {
	return "policyserv-keyring"
}

func (h *Homeserver) FetchKeys(ctx context.Context, requests map[gomatrixserverlib.PublicKeyLookupRequest]spec.Timestamp) (map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult, error) {
	log.Printf("FetchKeys: fetching keys: %+v", requests)
	results := make(map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult)

	toFetch := make(map[gomatrixserverlib.PublicKeyLookupRequest]spec.Timestamp)
	for req, ts := range requests {
		// Try to handle local requests first
		if req.ServerName == h.ServerName && req.KeyID == h.KeyId {
			results[req] = gomatrixserverlib.PublicKeyLookupResult{
				VerifyKey: gomatrixserverlib.VerifyKey{
					Key: spec.Base64Bytes(h.signingKey.Public().(ed25519.PublicKey)),
				},
				ExpiredTS:    gomatrixserverlib.PublicKeyNotExpired,
				ValidUntilTS: spec.AsTimestamp(time.Now().Add(signingKeyExpiration)),
			}
			continue // don't process further
		}

		// Then in-memory requests
		cachedServerKeys, found := h.keyCache.Get(string(req.ServerName))
		if found {
			if cachedForKeyId, ok := cachedServerKeys[string(req.KeyID)]; ok {
				if cachedForKeyId.WasValidAt(ts, gomatrixserverlib.StrictValiditySignatureCheck) {
					results[req] = cachedForKeyId
					continue // don't process further
				}
			}
		}

		// Then finally queue it for fetching
		toFetch[req] = ts
	}

	// Fetch any results we're missing
	toStore := make(map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult)
	keepResult := func(req gomatrixserverlib.PublicKeyLookupRequest, res gomatrixserverlib.PublicKeyLookupResult) {
		toStore[req] = res
		results[req] = res
		delete(toFetch, req)
	}
	for _, fetcher := range h.keyRing.KeyFetchers {
		if len(toFetch) == 0 {
			break // stop searching for keys
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		//goland:noinspection GoDeferInLoop
		defer cancel()

		fetched, err := fetcher.FetchKeys(timeoutCtx, toFetch)
		if err != nil {
			log.Printf("Non-fatal error fetching keys with %s: %v", h.FetcherName(), err)
			continue
		}

		for req, res := range fetched {
			// Store the longest-until-expiration result
			if prev, ok := results[req]; ok {
				if res.ValidUntilTS > prev.ValidUntilTS {
					keepResult(req, res)
				}
			} else {
				keepResult(req, res)
			}
		}
	}

	if len(toStore) > 0 {
		err := h.StoreKeys(ctx, toStore)
		if err != nil {
			log.Printf("Non-fatal error storing keys: %v", err)
		}
	}

	for req, ts := range requests {
		if _, ok := results[req]; !ok {
			log.Printf("Failed to fetch keys for %+v @ %d", req, ts)
		}
	}

	return results, nil
}

func (h *Homeserver) StoreKeys(ctx context.Context, results map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult) error {
	log.Printf("Storing signing key results: %+v", results)
	for req, result := range results {
		serverName := string(req.ServerName)
		keyId := string(req.KeyID)

		existing, found := h.keyCache.Get(serverName)
		if !found {
			existing = make(map[string]gomatrixserverlib.PublicKeyLookupResult)
		}
		existing[keyId] = result

		h.keyCache.Set(serverName, existing, cache.WithExpiration(cachedSigningKeyDuration))
	}
	return nil
}
