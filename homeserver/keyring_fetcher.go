package homeserver

import (
	"bytes"
	"context"
	"log"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/filter"
)

type ExcludeUnsafeKeysFetcher struct {
	downstream gomatrixserverlib.KeyFetcher
}

func NewExcludeUnsafeKeysFetcher(downstream gomatrixserverlib.KeyFetcher) *ExcludeUnsafeKeysFetcher {
	return &ExcludeUnsafeKeysFetcher{
		downstream: downstream,
	}
}

func (f *ExcludeUnsafeKeysFetcher) FetchKeys(ctx context.Context, reqs map[gomatrixserverlib.PublicKeyLookupRequest]spec.Timestamp) (map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult, error) {
	results, err := f.downstream.FetchKeys(ctx, reqs)
	if err != nil {
		return nil, err
	}
	return filterUnsafeKeysOut(results), nil
}

func (f *ExcludeUnsafeKeysFetcher) FetcherName() string {
	return "ExcludeUnsafeKeys_" + f.downstream.FetcherName()
}

func filterUnsafeKeysOut(results map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult) map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult {
	newResults := make(map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult)
	for req, res := range results {
		matched := false
		for _, unsafeKey := range filter.UnsafeSigningKeys() {
			if bytes.Equal(res.Key, unsafeKey) {
				log.Printf("Excluding unsafe signing key %s from %s", req.KeyID, req.ServerName)
				matched = true
				break
			}
		}
		if !matched {
			newResults[req] = res
		}
	}
	return newResults
}
