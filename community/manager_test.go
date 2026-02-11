package community

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func makeManager(t *testing.T) *Manager {
	db := test.NewMemoryStorage(t)
	assert.NotNil(t, db)

	pubsubClient := test.NewMemoryPubsub(t)
	assert.NotNil(t, pubsubClient)

	cnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cnf)

	manager, err := NewManager(cnf, db, pubsubClient, test.MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	return manager
}

func TestGetFilterSetForRoomId(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := makeManager(t)

	// Returns nil (and no error) when room isn't configured
	roomId := "!room:example.org"
	set, err := manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.Nil(t, set)

	// Normally this wouldn't be possible due to foreign key constraints, but memory datastores
	// don't have those, so let's test that we also get nil (and no error) when the community
	// doesn't exist, but room does.
	communityId := "test_community"
	err = manager.storage.UpsertRoom(ctx, &storage.StoredRoom{
		RoomId:                         roomId,
		RoomVersion:                    "11",
		ModeratorUserId:                "@moderator:example.org",
		LastCachedStateTimestampMillis: time.Now().UnixMilli(),
		CommunityId:                    communityId,
	})
	assert.NoError(t, err)
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.Nil(t, set)

	// Now add the community and ensure we get a returned set
	err = manager.storage.UpsertCommunity(ctx, &storage.StoredCommunity{
		CommunityId: communityId,
		Name:        "Test Community 1",
		Config: &config.CommunityConfig{
			KeywordFilterKeywords:    &[]string{"keyword1"},
			HellbanPostfilterMinutes: internal.Pointer(-1), // disable hellban to avoid it interfering with this test
		},
	})
	assert.NoError(t, err)
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Ensure our keyword trigger activates (proving the filter is configured correctly)
	keyword1Event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event1",
		RoomId:  roomId,
		Type:    "m.room.message",
		Sender:  "@test1:example.org",
		Content: map[string]interface{}{
			"body": "keyword1",
		},
	})
	vecs, err := set.CheckEvent(ctx, keyword1Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.True(t, set.IsSpamResponse(ctx, vecs))

	// Now, create a second room and community and ensure that the keyword1 event above
	// is *not* spam, but a new keyword *is* spam (proving we can have independent configs)
	roomId2 := "!second:example.org"
	communityId2 := "another_community"
	err = manager.storage.UpsertRoom(ctx, &storage.StoredRoom{
		RoomId:                         roomId2,
		RoomVersion:                    "11",
		ModeratorUserId:                "@moderator:example.org",
		LastCachedStateTimestampMillis: time.Now().UnixMilli(),
		CommunityId:                    communityId2,
	})
	assert.NoError(t, err)
	err = manager.storage.UpsertCommunity(ctx, &storage.StoredCommunity{
		CommunityId: communityId2,
		Name:        "Test Community 2",
		Config: &config.CommunityConfig{
			KeywordFilterKeywords:    &[]string{"keyword2"},
			HellbanPostfilterMinutes: internal.Pointer(-1), // disable hellban to avoid it interfering with this test
		},
	})
	assert.NoError(t, err)
	set, err = manager.GetFilterSetForRoomId(ctx, roomId2)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	vecs, err = set.CheckEvent(ctx, keyword1Event, nil)
	assert.NoError(t, err)
	assert.False(t, set.IsSpamResponse(ctx, vecs))
	keyword2Event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event2",
		RoomId:  roomId,
		Type:    "m.room.message",
		Sender:  "@test2:example.org",
		Content: map[string]interface{}{
			"body": "keyword2",
		},
	})
	vecs, err = set.CheckEvent(ctx, keyword2Event, nil)
	assert.NoError(t, err)
	assert.True(t, set.IsSpamResponse(ctx, vecs))
}

func TestCommunityManagerCacheInvalidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := makeManager(t)

	roomId := "!room:example.org"
	communityId := "test_community"

	// Create resources first
	err := manager.storage.UpsertRoom(ctx, &storage.StoredRoom{
		RoomId:                         roomId,
		RoomVersion:                    "11",
		ModeratorUserId:                "@moderator:example.org",
		LastCachedStateTimestampMillis: time.Now().UnixMilli(),
		CommunityId:                    communityId,
	})
	assert.NoError(t, err)
	cnf := &config.CommunityConfig{
		KeywordFilterKeywords:    &[]string{"keyword1"},
		HellbanPostfilterMinutes: internal.Pointer(-1), // disable hellban to avoid it interfering with this test
	}
	err = manager.storage.UpsertCommunity(ctx, &storage.StoredCommunity{
		CommunityId: communityId,
		Name:        "Test Community 1",
		Config:      cnf,
	})
	assert.NoError(t, err)

	// Ensure the caches get populated
	retCommunityId, err := manager.getCommunityIdForRoom(ctx, roomId)
	assert.NoError(t, err)
	assert.Equal(t, communityId, retCommunityId)
	set, err := manager.GetFilterSetForCommunityId(ctx, communityId)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Ensure the keyword config was picked up
	keyword1Event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event1",
		RoomId:  roomId,
		Type:    "m.room.message",
		Sender:  "@test1:example.org",
		Content: map[string]interface{}{
			"body": "keyword1",
		},
	})
	vecs, err := set.CheckEvent(ctx, keyword1Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.True(t, set.IsSpamResponse(ctx, vecs))

	// Now, change the config but *don't* notify the manager about it to prove the cache is working
	cnf.KeywordFilterKeywords = &[]string{"keyword2"}
	err = manager.storage.UpsertCommunity(ctx, &storage.StoredCommunity{
		CommunityId: communityId,
		Name:        "Test Community 1",
		Config:      cnf,
	})
	assert.NoError(t, err)
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	vecs, err = set.CheckEvent(ctx, keyword1Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.True(t, set.IsSpamResponse(ctx, vecs)) // should still be spam

	// Now, notify the manager and re-test
	err = manager.pubsubClient.Publish(ctx, pubsub.TopicCommunityConfig, communityId)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second) // wait a bit for it to pick up the change
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	vecs, err = set.CheckEvent(ctx, keyword1Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.False(t, set.IsSpamResponse(ctx, vecs)) // should not be spam

	// Create a second community and assign the existing room to it, but again: don't
	// notify the manager of that room ID to community ID change
	communityId2 := "another_community"
	err = manager.storage.UpsertCommunity(ctx, &storage.StoredCommunity{
		CommunityId: communityId2,
		Name:        "Test Community 2",
		Config: &config.CommunityConfig{
			KeywordFilterKeywords:    &[]string{"keyword3"},
			HellbanPostfilterMinutes: internal.Pointer(-1), // disable hellban to avoid it interfering with this test
		},
	})
	assert.NoError(t, err)
	err = manager.storage.UpsertRoom(ctx, &storage.StoredRoom{
		RoomId:                         roomId,
		RoomVersion:                    "11",
		ModeratorUserId:                "@moderator:example.org",
		LastCachedStateTimestampMillis: time.Now().UnixMilli(),
		CommunityId:                    communityId2,
	})
	assert.NoError(t, err)
	keyword3Event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event3",
		RoomId:  roomId,
		Type:    "m.room.message",
		Sender:  "@test3:example.org",
		Content: map[string]interface{}{
			"body": "keyword3",
		},
	})
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	vecs, err = set.CheckEvent(ctx, keyword3Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.False(t, set.IsSpamResponse(ctx, vecs))

	// Now notify the manager and re-check the event (which should now be spam)
	err = manager.pubsubClient.Publish(ctx, pubsub.TopicRoomCommunityId, roomId)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second) // wait a bit for it to pick up the change
	set, err = manager.GetFilterSetForRoomId(ctx, roomId)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	vecs, err = set.CheckEvent(ctx, keyword3Event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.True(t, set.IsSpamResponse(ctx, vecs)) // now it's spam
}
