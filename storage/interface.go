package storage

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
)

type StoredRoom struct {
	RoomId                         string `json:"room_id"`
	RoomVersion                    string `json:"room_version"`
	ModeratorUserId                string `json:"moderator_user_id"` // TODO: Drop
	LastCachedStateTimestampMillis int64  `json:"last_cached_state_timestamp"`
	CommunityId                    string `json:"community_id"`
}

type StoredEventResult struct {
	EventId        string             `json:"event_id"`
	IsProbablySpam bool               `json:"is_probably_spam"`
	ContentInfo    *harms.ContentInfo `json:"-"` // can't be exported to/imported from JSON
}

type StoredCommunity struct {
	CommunityId      string                  `json:"community_id"`
	Name             string                  `json:"name"`
	Config           *config.CommunityConfig `json:"config"`
	ApiAccessToken   *string                 `json:"-"` // don't export to/import from JSON
	CanSelfJoinRooms bool                    `json:"can_self_join_rooms"`
}

type StateLearnQueueItem struct {
	RoomId               string
	AtEventId            string
	ViaServer            string
	AfterTimestampMillis int64
}

type StoredKeywordTemplate struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type StoredMediaClassification struct {
	MxcUri          string
	CommunityId     string
	Classifications StoredClassifications
}

type StoredEdu struct {
	Destination string
	Payload     gomatrixserverlib.EDU
}

type MatrixTransaction struct {
	TransactionId string
	Destination   string
	Edus          []gomatrixserverlib.EDU
}

// StoredClassifications implements the SQL driver interface for scanning/setting values. Note that this used to
// be a []classification.Classification type, but was modernized to use harms instead. Legacy data is still possible
// and we need to maintain backwards compatibility, so we go out of our way to reinterpret the simple array into a
// harms.ContentInfo struct.
// Note that difference receivers are used for Value and Scan - this is because Go's pointers are expecting certain
// values depending on whether it's being read or written.
type StoredClassifications struct {
	*harms.ContentInfo
}

func (c StoredClassifications) Value() (driver.Value, error) {
	strArray := []string{c.ContentInfo.Class().String()} // put the content class first for easy decoding
	for _, h := range c.ContentInfo.Harms() {
		strArray = append(strArray, string(h))
	}
	return json.Marshal(strArray)
}

func (c *StoredClassifications) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return nil
	}
	strArray := make([]string, 0)
	err := json.Unmarshal(b, &strArray)
	if err != nil {
		return err
	}
	if len(strArray) > 0 {
		if strArray[0] == harms.ContentClassNeutral.String() {
			if len(strArray) != 1 {
				return fmt.Errorf("invalid neutral content classification: %v", strArray)
			}
			c.ContentInfo = harms.NeutralContent()
		} else if strArray[0] == harms.ContentClassAllowed.String() {
			if len(strArray) != 1 {
				return fmt.Errorf("invalid allowed content classification: %v", strArray)
			}
			c.ContentInfo = harms.AllowedContent()
		} else {
			// Assume it's prohibited content
			if strArray[0] == harms.ContentClassProhibited.String() {
				strArray = strArray[1:] // remove the content class if it was specified
			}
			harmIds := make([]harms.Harm, 0)
			for _, h := range strArray {
				harmIds = append(harmIds, harms.Harm(h))
			}
			c.ContentInfo = harms.ProhibitedContent(harmIds...)
		}
	} else {
		c.ContentInfo = harms.NeutralContent()
	}
	return nil
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
	GetCommunityByAccessToken(ctx context.Context, accessToken string) (*StoredCommunity, error)

	// PopStateLearnQueue - returns the next item in the state learn queue, or nil if the queue is empty.
	// The caller is responsible for calling Commit() on the returned Transaction, completing the operation.
	PopStateLearnQueue(ctx context.Context) (*StateLearnQueueItem, Transaction, error)
	PushStateLearnQueue(ctx context.Context, item *StateLearnQueueItem) error

	// SetTrustData - stores arbitrary "trust" data under a given key. The key is scoped to the sourceName, and may be
	// an empty string. The data is stored as JSON and must be serializable.
	SetTrustData(ctx context.Context, sourceName string, key string, data any) error
	GetTrustData(ctx context.Context, sourceName string, key string, result any) error

	UpsertKeywordTemplate(ctx context.Context, template *StoredKeywordTemplate) error
	GetKeywordTemplate(ctx context.Context, name string) (*StoredKeywordTemplate, error)

	UpsertMediaClassification(ctx context.Context, classification *StoredMediaClassification) error
	GetMediaClassification(ctx context.Context, mxcUri string, communityId string) (*StoredMediaClassification, error)

	// BeginMatrixTransaction - pulls the data required to send (over federation) a transaction of data to a destination.
	// The caller is responsible for calling Commit() on the returned SQL Transaction to indicate that the MatrixTransaction
	// was successfully sent. This locks the destination to prevent concurrent sends. If no data is to be sent to the destination,
	// then this returns an sql.ErrNoRows error (with a nil Transaction and nil MatrixTransaction).
	BeginMatrixTransaction(ctx context.Context, destination string) (*MatrixTransaction, Transaction, error)
	InsertEdu(ctx context.Context, edu *StoredEdu) error // note: not an Upsert operation
	GetDestinationsNeedingCatchup(ctx context.Context) ([]string, error)
}
