package community

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/filter"
	"github.com/matrix-org/policyserv/filter/audit"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

type Manager struct {
	storage              storage.PersistentStorage
	communityFilterCache *cache.Cache[string, *filter.Set] // community ID -> filter set
	roomToCommunityCache *cache.Cache[string, string]      // room ID -> community ID
	instanceConfig       *config.InstanceConfig
	pubsubClient         pubsub.Client
	auditQueue           *audit.Queue
}

func NewManager(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, pubsubClient pubsub.Client, auditQueue *audit.Queue) (*Manager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	filterCache := cache.New[string, *filter.Set]() // we don't use a janitor because the filters may need a proper shutdown
	configCh, err := pubsubClient.Subscribe(ctx, pubsub.TopicCommunityConfig)
	if err != nil {
		return nil, err
	}
	go invalidateCacheOnChannel(filterCache, "community filters", configCh, func(communityId string, set *filter.Set) {
		err := set.Close()
		if err != nil {
			log.Printf("Error closing filter set for %s: %v", communityId, err)
		}
	})

	roomIdCache := cache.New[string, string](
		// This should be similarly rare to change, so can be cleaned up slowly
		cache.WithJanitorInterval[string, string](15 * time.Minute),
	)
	roomIdCh, err := pubsubClient.Subscribe(ctx, pubsub.TopicRoomCommunityId)
	if err != nil {
		return nil, err
	}
	go invalidateCacheOnChannel(roomIdCache, "room ID to community ID", roomIdCh, nil)

	return &Manager{
		storage:              storage,
		communityFilterCache: filterCache,
		roomToCommunityCache: roomIdCache,
		instanceConfig:       instanceConfig,
		pubsubClient:         pubsubClient,
		auditQueue:           auditQueue,
	}, nil
}

func invalidateCacheOnChannel[V any](cacheInstance *cache.Cache[string, V], cacheName string, ch <-chan string, expireFn func(key string, val V)) {
	for val := range ch {
		if val == pubsub.ClosingValue {
			return // stop getting values
		}

		log.Printf("Invalidating entry %s in '%s' cache given change", val, cacheName)
		if expireFn != nil {
			if cacheVal, ok := cacheInstance.Get(val); ok {
				expireFn(val, cacheVal)
			}
		}
		cacheInstance.Delete(val)
	}
}

func (m *Manager) GetFilterSetForCommunityId(ctx context.Context, communityId string) (*filter.Set, error) {
	fromCache, ok := m.communityFilterCache.Get(communityId)
	if ok {
		return fromCache, nil
	}

	log.Printf("Creating net-new filter set for %s", communityId)

	communityConfig, err := m.getCommunityConfig(ctx, communityId)
	if err != nil {
		return nil, err
	}
	if communityConfig == nil {
		return nil, nil
	}

	prefilters := make([]string, 0)
	hellbanPrefilters := make([]string, 0) // these run after the prefilters, but before the other filters
	filters := make([]string, 0)
	postfilters := make([]string, 0)
	if len(internal.Dereference(communityConfig.KeywordFilterKeywords)) > 0 {
		filters = append(filters, filter.KeywordFilterName)
	}
	if len(internal.Dereference(communityConfig.KeywordTemplateFilterTemplateNames)) > 0 {
		filters = append(filters, filter.KeywordTemplateFilterName)
	}
	if len(internal.Dereference(communityConfig.EventTypePrefilterAllowedEventTypes)) > 0 || len(internal.Dereference(communityConfig.EventTypePrefilterAllowedStateEventTypes)) > 0 {
		prefilters = append(prefilters, filter.EventTypeFilterName)
	}
	if len(internal.Dereference(communityConfig.SenderPrefilterAllowedSenders)) > 0 {
		prefilters = append(prefilters, filter.SenderFilterName)
	}
	if internal.Dereference(communityConfig.DensityFilterMaxDensity) > 0 {
		filters = append(filters, filter.DensityFilterName)
	}
	if internal.Dereference(communityConfig.LengthFilterMaxLength) > 0 {
		filters = append(filters, filter.LengthFilterName)
	}
	if internal.Dereference(communityConfig.ManyAtsFilterMaxAts) > 0 {
		filters = append(filters, filter.ManyAtsFilterName)
	}
	if len(internal.Dereference(communityConfig.MediaFilterMediaTypes)) > 0 {
		filters = append(filters, filter.MediaFilterName)
	}
	if len(internal.Dereference(communityConfig.UntrustedMediaFilterMediaTypes)) > 0 {
		filters = append(filters, filter.UntrustedMediaFilterName)
	}
	if internal.Dereference(communityConfig.MentionFilterMaxMentions) > 0 {
		filters = append(filters, filter.MentionsFilterName)
	}
	if internal.Dereference(communityConfig.MjolnirFilterEnabled) && m.instanceConfig.MjolnirFilterRoomID != "" {
		filters = append(filters, filter.MjolnirFilterName)
	}
	if internal.Dereference(communityConfig.TrimLengthFilterMaxDifference) > 0 {
		filters = append(filters, filter.TrimLengthFilterName)
	}
	if internal.Dereference(communityConfig.HellbanPostfilterMinutes) > 0 {
		hellbanPrefilters = append(hellbanPrefilters, filter.HellbanPrefilterName)
		postfilters = append(postfilters, filter.HellbanPostfilterName)
	}
	if m.instanceConfig.OpenAIApiKey != "" {
		// Access to this filter is gated by further instance config (namely, the room IDs allowed to use it)
		filters = append(filters, filter.OpenAIOmniFilterName)
	}
	if !internal.Dereference(communityConfig.StickyEventsFilterAllowStickyEvents) {
		filters = append(filters, filter.StickyEventsFilterName)
	}
	if len(internal.Dereference(communityConfig.LinkFilterAllowedUrlGlobs)) > 0 || len(internal.Dereference(communityConfig.LinkFilterDeniedUrlGlobs)) > 0 {
		filters = append(filters, filter.LinkFilterName)
	}
	var scanner content.Scanner
	if m.instanceConfig.HMAApiUrl != "" && len(internal.Dereference(communityConfig.HMAFilterEnabledBanks)) > 0 {
		filters = append(filters, filter.MediaScanningFilterName)
		scanner, err = content.NewHMAScanner(m.instanceConfig.HMAApiUrl, m.instanceConfig.HMAApiKey, internal.Dereference(communityConfig.HMAFilterEnabledBanks))
		if err != nil {
			return nil, fmt.Errorf("failed to create HMA scanner: %w", err)
		}
	}
	if communityConfig.UnsafeSigningKeyFilterEnabled {
		prefilters = append(prefilters, filter.UnsafeSigningKeyFilterName)
	}
	setConfig := &filter.SetConfig{
		CommunityConfig: communityConfig,
		CommunityId:     communityId,
		InstanceConfig:  m.instanceConfig,
		Groups: []*filter.SetGroupConfig{{
			// The first set group replaces the concept of "prefilters". We want this group to capture all events,
			// so we set the min and max to cover the full range.
			EnabledNames:           prefilters,
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}, {
			// This set group is similar to prefilters, but only contains the hellban prefilters. This is to prevent
			// users being denied abilities that are intended to be granted to them via other prefilters (like an
			// ability to leave the room).
			EnabledNames: hellbanPrefilters,
			// We want the min/max values to catch "maybe spam" from the prefilters level, but explicitly not events
			// that were flagged as not-spam by the same layer. This is why the minimum is not quite zero.
			MinimumSpamVectorValue: 0.1,
			MaximumSpamVectorValue: 1.0,
		}, {
			// The third set group is what was previously the middle layer for filters. This is where the bulk of
			// the work happens. We only want it to run if the prefilters didn't already declare an event spammy or
			// neutral though, so we narrow the min/max range a bit.
			EnabledNames: filters,
			// We want the min/max to be slightly less than the extremes to avoid doing work
			// when the event is already flagged as (not) spam.
			MinimumSpamVectorValue: 0.3,
			MaximumSpamVectorValue: 0.7,
		}, {
			// The last set group replaces "postfilters", and should be run regardless of whether the previous groups
			// flagged the event as spam. As such, we set the min/max to the widest possible range.
			EnabledNames:           postfilters,
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	filterSet, err := filter.NewSet(setConfig, m.storage, m.pubsubClient, m.auditQueue, scanner)
	if err != nil {
		return nil, err
	}

	m.communityFilterCache.Set(communityId, filterSet) // we don't expire these entries

	return filterSet, nil
}

func (m *Manager) getCommunityConfig(ctx context.Context, communityId string) (*config.CommunityConfig, error) {
	// Find JSON in database
	community, err := m.storage.GetCommunity(ctx, communityId)
	if err != nil {
		return nil, err
	}
	if community == nil {
		return nil, nil
	}

	// Bring the config down to bytes for layering
	b, err := json.Marshal(community.Config)
	if err != nil {
		return nil, err
	}

	cnf, err := config.NewCommunityConfigForJSON(b)
	if err != nil {
		return nil, err
	}

	return cnf, nil
}

func (m *Manager) getCommunityIdForRoom(ctx context.Context, roomId string) (string, error) {
	fromCache, ok := m.roomToCommunityCache.Get(roomId)
	if ok {
		return fromCache, nil
	}

	room, err := m.storage.GetRoom(ctx, roomId)
	if err != nil {
		return "", err
	}
	if room == nil {
		return "", nil
	}

	// It's rare that these things change, so hold them for long amounts of time in the cache.
	// We *should* also expire values from the cache upon change, but this is just in case that doesn't happen.
	m.roomToCommunityCache.Set(roomId, room.CommunityId, cache.WithExpiration(60*time.Minute))

	return room.CommunityId, nil
}

func (m *Manager) GetFilterSetForRoomId(ctx context.Context, roomId string) (*filter.Set, error) {
	communityId, err := m.getCommunityIdForRoom(ctx, roomId)
	if err != nil {
		return nil, err
	}
	if communityId == "" {
		return nil, nil
	}

	return m.GetFilterSetForCommunityId(ctx, communityId)
}
