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
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/notifiers"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

type Manager struct {
	storage              storage.PersistentStorage
	communityFilterCache *cache.Cache[string, *filter.Set] // community ID -> filter set
	roomToCommunityCache *cache.Cache[string, string]      // room ID -> community ID
	instanceConfig       *config.InstanceConfig
	pubsubClient         pubsub.Client
	notifier             notifiers.MatrixNotifier
}

func NewManager(instanceConfig *config.InstanceConfig, storage storage.PersistentStorage, pubsubClient pubsub.Client, notifier notifiers.MatrixNotifier) (*Manager, error) {
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
		notifier:             notifier,
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

	prefilters := []string{filter.ProtectLocalUserFilterName}
	hellbanPrefilters := make([]string, 0) // these run after the prefilters, but before the other filters
	filters := make([]string, 0)
	postfilterSilences := make([]string, 0)
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
	if internal.Dereference(communityConfig.InlineEmojiSizeFilterMaxHeightPixels) > 0 {
		filters = append(filters, filter.InlineEmojiSizeFilterName)
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
		postfilterSilences = append(postfilterSilences, filter.HellbanPostfilterName)
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
	if internal.Dereference(communityConfig.MentionFrequencyFilterRateLimit) > 0 {
		filters = append(filters, filter.MentionsFrequencyFilterName)
	}
	if len(internal.Dereference(communityConfig.FrequencyFilterEventTypes)) > 0 && internal.Dereference(communityConfig.FrequencyFilterRateLimit) > 0 {
		filters = append(filters, filter.FrequencyFilterName)
	}
	if internal.Dereference(communityConfig.UserIdContainsWordsFilterMaxWords) > 0 {
		filters = append(filters, filter.UserIdContainsWordsFilterName)
	}
	if internal.Dereference(communityConfig.UserIdLengthFilterMaxLength) > 0 {
		filters = append(filters, filter.UserIdLengthFilterName)
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
			// so we set the classes appropriately.
			EnabledNames: prefilters,
			// Micro optimization: Put neutral first so we can skip CPU cycles in a loop in the general case. We also
			// probably don't need to specify allowed and prohibited here because we have no code which pushes such
			// classes right away, but for safety we might as well.
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral, harms.ContentClassAllowed, harms.ContentClassProhibited},
		}, {
			// This set group is similar to prefilters, but only contains the hellban prefilters. This is to prevent
			// users being denied abilities that are intended to be granted to them via other prefilters (like an
			// ability to leave the room).
			EnabledNames: hellbanPrefilters,
			// We want to capture "maybe spam", but not events that were already flagged as (not) spam.
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral},
		}, {
			// The third set group is what was previously the middle layer for filters. This is where the bulk of
			// the work happens. We only want it to run if the prefilters didn't already declare an event spammy or
			// neutral though, so we narrow the min/max range a bit.
			EnabledNames: filters,
			// Skip this group for events that are already (not) spam.
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral},
		}, {
			// The last set group is for telling the hellban postfilter if any previous filter flagged an event as
			// spammy so it can put a silence in place.
			EnabledNames: postfilterSilences,
			RunOnClasses: []harms.ContentClass{harms.ContentClassProhibited},
		}},
	}
	filterSet, err := filter.NewSet(setConfig, m.storage, m.pubsubClient, m.notifier, scanner)
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
