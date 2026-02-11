package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/storage"
	"github.com/ryanuber/go-glob"
	"github.com/stretchr/testify/assert"
)

var SimulatedError = errors.New("simulated error")

const ErrorEventResultId = "$ERROR"

type MemoryStorage struct {
	t                      *testing.T
	rooms                  map[string]*storage.StoredRoom
	events                 map[string]*storage.StoredEventResult
	userIdsDisplayNames    map[string][][]string        // roomId -> [userIds, displayNames]
	policyRules            map[string]map[string]string // roomId -> entity(userId, server name, etc) -> entityType(m.policy.rule.user, etc)
	communities            map[string]*storage.StoredCommunity
	learnStateQueue        []*storage.StateLearnQueueItem
	pendingLearnStateQueue []*storage.StateLearnQueueItem
	trustData              map[string]map[string][]byte // sourceName -> key -> JSON value
	keywordTemplates       map[string]*storage.StoredKeywordTemplate
}

func NewMemoryStorage(t *testing.T) *MemoryStorage {
	return &MemoryStorage{
		t:                      t,
		rooms:                  make(map[string]*storage.StoredRoom),
		events:                 make(map[string]*storage.StoredEventResult),
		userIdsDisplayNames:    make(map[string][][]string),
		policyRules:            make(map[string]map[string]string),
		communities:            make(map[string]*storage.StoredCommunity),
		learnStateQueue:        make([]*storage.StateLearnQueueItem, 0),
		pendingLearnStateQueue: make([]*storage.StateLearnQueueItem, 0),
		trustData:              make(map[string]map[string][]byte),
		keywordTemplates:       make(map[string]*storage.StoredKeywordTemplate),
	}
}

func (m *MemoryStorage) Close() error {
	// no-op
	return nil
}

func (m *MemoryStorage) GetAllRooms(ctx context.Context) ([]*storage.StoredRoom, error) {
	assert.NotNil(m.t, ctx, "context is required")

	rooms := make([]*storage.StoredRoom, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func (m *MemoryStorage) GetRoom(ctx context.Context, roomId string) (*storage.StoredRoom, error) {
	assert.NotNil(m.t, ctx, "context is required")

	return m.rooms[roomId], nil
}

func (m *MemoryStorage) UpsertRoom(ctx context.Context, room *storage.StoredRoom) error {
	assert.NotNil(m.t, ctx, "context is required")

	m.rooms[room.RoomId] = room
	return nil
}

func (m *MemoryStorage) DeleteRoom(ctx context.Context, roomId string) error {
	assert.NotNil(m.t, ctx, "context is required")

	delete(m.rooms, roomId)
	return nil
}

func (m *MemoryStorage) GetEventResult(ctx context.Context, eventId string) (*storage.StoredEventResult, error) {
	assert.NotNil(m.t, ctx, "context is required")

	if eventId == ErrorEventResultId {
		return nil, SimulatedError
	}

	return m.events[eventId], nil
}

func (m *MemoryStorage) UpsertEventResult(ctx context.Context, event *storage.StoredEventResult) error {
	assert.NotNil(m.t, ctx, "context is required")

	m.events[event.EventId] = event
	return nil
}

func (m *MemoryStorage) GetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string) ([]string, []string, error) {
	assert.NotNil(m.t, ctx, "context is required")

	arr := m.userIdsDisplayNames[roomId]
	if arr == nil {
		return []string{}, []string{}, nil
	}
	return arr[0], arr[1], nil
}

func (m *MemoryStorage) SetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string, userIds []string, displayNames []string) error {
	assert.NotNil(m.t, ctx, "context is required")

	m.userIdsDisplayNames[roomId] = [][]string{userIds, displayNames}
	return nil
}

func (m *MemoryStorage) IsUserBannedInList(ctx context.Context, listRoomId string, userId string) (bool, error) {
	assert.NotNil(m.t, ctx, "context is required")

	rules := m.policyRules[listRoomId]
	if rules == nil {
		return false, nil
	}

	parsedUserId, err := spec.NewUserID(userId, true)
	if err != nil {
		return false, err
	}

	for entityId, entityType := range rules {
		entity := parsedUserId.String()
		if entityType == "m.policy.rule.server" {
			entity = string(parsedUserId.Domain())
		}
		if glob.Glob(entityId, entity) {
			return true, nil
		}
	}

	return false, nil
}

func (m *MemoryStorage) SetListBanRules(ctx context.Context, listRoomId string, entityToEntityType map[string]string) error {
	assert.NotNil(m.t, ctx, "context is required")

	m.policyRules[listRoomId] = entityToEntityType
	return nil
}

func (m *MemoryStorage) CreateCommunity(ctx context.Context, name string) (*storage.StoredCommunity, error) {
	assert.NotNil(m.t, ctx, "context is required")

	community := &storage.StoredCommunity{
		CommunityId: storage.NextId(),
		Name:        name,
		Config:      &config.CommunityConfig{}, // empty by default
	}
	assert.NotEmpty(m.t, community.CommunityId)

	// We clone to prevent mutations causing the storage to also be updated
	m.communities[community.CommunityId] = mustClone(m.t, community)
	return community, nil
}

func (m *MemoryStorage) GetCommunity(ctx context.Context, communityId string) (*storage.StoredCommunity, error) {
	assert.NotNil(m.t, ctx, "context is required")

	// We clone to prevent mutations causing the storage to also be updated
	return mustClone(m.t, m.communities[communityId]), nil
}

func (m *MemoryStorage) UpsertCommunity(ctx context.Context, community *storage.StoredCommunity) error {
	assert.NotNil(m.t, ctx, "context is required")
	// We clone to prevent mutations causing the storage to also be updated
	m.communities[community.CommunityId] = mustClone(m.t, community)
	return nil
}

func (m *MemoryStorage) PushStateLearnQueue(ctx context.Context, item *storage.StateLearnQueueItem) error {
	assert.NotNil(m.t, ctx, "context is required")
	m.learnStateQueue = append(m.learnStateQueue, item)
	return nil
}

func (m *MemoryStorage) PopStateLearnQueue(ctx context.Context) (*storage.StateLearnQueueItem, storage.Transaction, error) {
	assert.NotNil(m.t, ctx, "context is required")
	if len(m.learnStateQueue) == 0 {
		return nil, nil, nil
	}
	item := m.learnStateQueue[0]
	m.learnStateQueue = m.learnStateQueue[1:]
	m.pendingLearnStateQueue = append(m.pendingLearnStateQueue, item)
	return item, &MemoryTransaction{storage: m, row: item}, nil
}

func (m *MemoryStorage) GetTrustData(ctx context.Context, sourceName string, key string, result any) error {
	assert.NotNil(m.t, ctx, "context is required")

	bySource, ok := m.trustData[sourceName]
	if !ok {
		return sql.ErrNoRows
	}

	val, ok := bySource[key]
	if !ok {
		return sql.ErrNoRows
	}

	return json.Unmarshal(val, result)
}

func (m *MemoryStorage) SetTrustData(ctx context.Context, sourceName string, key string, data any) error {
	assert.NotNil(m.t, ctx, "context is required")
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if m.trustData[sourceName] == nil {
		m.trustData[sourceName] = make(map[string][]byte)
	}
	m.trustData[sourceName][key] = val
	return nil
}

func (m *MemoryStorage) GetKeywordTemplate(ctx context.Context, name string) (*storage.StoredKeywordTemplate, error) {
	assert.NotNil(m.t, ctx, "context is required")

	val, ok := m.keywordTemplates[name]
	if !ok {
		return nil, sql.ErrNoRows
	}

	return val, nil
}

func (m *MemoryStorage) UpsertKeywordTemplate(ctx context.Context, template *storage.StoredKeywordTemplate) error {
	assert.NotNil(m.t, ctx, "context is required")
	m.keywordTemplates[template.Name] = template
	return nil
}

func mustClone[T any](t *testing.T, val *T) *T {
	if val == nil {
		return nil
	}

	raw := *val
	raw2 := raw // this is where the clone happens. See https://stackoverflow.com/a/51638160
	cloned := &raw2
	assert.False(t, cloned == val)
	assert.Equal(t, val, cloned)
	return cloned
}

type MemoryTransaction struct { // Implements storage.Transaction
	storage *MemoryStorage
	row     *storage.StateLearnQueueItem
}

func (t *MemoryTransaction) Commit() error {
	newQueue := make([]*storage.StateLearnQueueItem, 0)
	for _, item := range t.storage.pendingLearnStateQueue {
		if item != t.row {
			newQueue = append(newQueue, item)
		}
	}
	t.storage.pendingLearnStateQueue = newQueue
	return nil
}

func (t *MemoryTransaction) Rollback() error {
	t.storage.learnStateQueue = append(t.storage.learnStateQueue, t.row)
	return t.Commit() // we're cheating a bit to avoid code duplication
}
