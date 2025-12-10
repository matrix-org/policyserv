package storage

import (
	"context"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/confidence"
)

type StoredRoom struct {
	RoomId                         string `json:"room_id"`
	RoomVersion                    string `json:"room_version"`
	ModeratorUserId                string `json:"moderator_user_id"` // TODO: Drop
	LastCachedStateTimestampMillis int64  `json:"last_cached_state_timestamp"`
	CommunityId                    string `json:"community_id"`
}

type StoredEventResult struct {
	EventId           string             `json:"event_id"`
	IsProbablySpam    bool               `json:"is_probably_spam"`
	ConfidenceVectors confidence.Vectors `json:"confidence_vectors"`
}

type StoredCommunity struct {
	CommunityId string                  `json:"community_id"`
	Name        string                  `json:"name"`
	Config      *config.CommunityConfig `json:"config"`
}

type StateLearnQueueItem struct {
	RoomId               string
	AtEventId            string
	ViaServer            string
	AfterTimestampMillis int64
}

type Transaction interface { // mirror of sql.Tx interface for ease of compatibility
	Commit() error
	Rollback() error
}

type PersistentStorage interface {
	Close() error

	GetAllRooms(ctx context.Context) ([]*StoredRoom, error)
	GetRoom(ctx context.Context, roomId string) (*StoredRoom, error)
	UpsertRoom(ctx context.Context, room *StoredRoom) error
	DeleteRoom(ctx context.Context, roomId string) error

	GetEventResult(ctx context.Context, eventId string) (*StoredEventResult, error)
	UpsertEventResult(ctx context.Context, event *StoredEventResult) error

	// GetUserIdsAndDisplayNamesByRoomId - returns (userIds, displayNames, error) for user IDs joined to the room.
	// Values are deduplicated.
	GetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string) ([]string, []string, error)
	// SetUserIdsAndDisplayNamesByRoomId - replaces the stored user IDs and display names for a given room. The supplied
	// slices MUST be the same length, and ordered against the user IDs slice.
	SetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string, userIds []string, displayNames []string) error

	IsUserBannedInList(ctx context.Context, listRoomId string, userId string) (bool, error)
	// SetListBanRules - sets the listRoomId's ban rules to the entityToEntityType map.
	// Example entityToEntityMap: {"@user:example.org":"m.policy.rule.user"}
	SetListBanRules(ctx context.Context, listRoomId string, entityToEntityType map[string]string) error

	CreateCommunity(ctx context.Context, name string) (*StoredCommunity, error)
	UpsertCommunity(ctx context.Context, community *StoredCommunity) error
	GetCommunity(ctx context.Context, id string) (*StoredCommunity, error)

	// PopStateLearnQueue - returns the next item in the state learn queue, or nil if the queue is empty.
	// The caller is responsible for calling Commit() on the returned Transaction, completing the operation.
	PopStateLearnQueue(ctx context.Context) (*StateLearnQueueItem, Transaction, error)
	PushStateLearnQueue(ctx context.Context, item *StateLearnQueueItem) error

	// SetTrustData - stores arbitrary "trust" data under a given key. The key is scoped to the sourceName, and may be
	// an empty string. The data is stored as JSON and must be serializable.
	SetTrustData(ctx context.Context, sourceName string, key string, data any) error
	GetTrustData(ctx context.Context, sourceName string, key string, result any) error
}
