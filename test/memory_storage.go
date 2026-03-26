package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/ryanuber/go-glob"
	"github.com/stretchr/testify/assert"
)

var SimulatedError = errors.New("simulated error")

const ErrorEventResultId = "$ERROR"

type memoryDestinationEdu struct {
	*storage.StoredEdu
	transactionId *string
}

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
	mediaClassifications   map[string]map[string]*storage.StoredMediaClassification // mxcUri -> communityId -> classification
	destinationLocks       map[string]*sync.Mutex
	destinationEdus        map[string][]*memoryDestinationEdu
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
		mediaClassifications:   make(map[string]map[string]*storage.StoredMediaClassification),
		destinationLocks:       make(map[string]*sync.Mutex),
		destinationEdus:        make(map[string][]*memoryDestinationEdu),
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

func (m *MemoryStorage) GetCommunityByAccessToken(ctx context.Context, accessToken string) (*storage.StoredCommunity, error) {
	assert.NotNil(m.t, ctx, "context is required")

	for _, community := range m.communities {
		if internal.Dereference(community.ApiAccessToken) == accessToken {
			// We clone to prevent mutations causing the storage to also be updated
			return mustClone(m.t, community), nil
		}
	}

	return nil, nil
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
	return item, &memoryStateLearnTransaction{storage: m, row: item}, nil
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

func (m *MemoryStorage) GetMediaClassification(ctx context.Context, mxcUri string, communityId string) (*storage.StoredMediaClassification, error) {
	assert.NotNil(m.t, ctx, "context is required")

	byCommunity, ok := m.mediaClassifications[mxcUri]
	if !ok {
		return nil, sql.ErrNoRows
	}
	val, ok := byCommunity[communityId]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return val, nil
}

func (m *MemoryStorage) UpsertMediaClassification(ctx context.Context, classification *storage.StoredMediaClassification) error {
	assert.NotNil(m.t, ctx, "context is required")

	if m.mediaClassifications[classification.MxcUri] == nil {
		m.mediaClassifications[classification.MxcUri] = make(map[string]*storage.StoredMediaClassification)
	}
	m.mediaClassifications[classification.MxcUri][classification.CommunityId] = classification
	return nil
}

func (m *MemoryStorage) InsertEdu(ctx context.Context, edu *storage.StoredEdu) error {
	assert.NotNil(m.t, ctx, "context is required")

	// Note: inserting happens without locking

	if m.destinationLocks[edu.Destination] == nil {
		m.destinationLocks[edu.Destination] = &sync.Mutex{}
	}
	if m.destinationEdus[edu.Destination] == nil {
		m.destinationEdus[edu.Destination] = make([]*memoryDestinationEdu, 0)
	}
	m.destinationEdus[edu.Destination] = append(m.destinationEdus[edu.Destination], &memoryDestinationEdu{
		StoredEdu:     edu,
		transactionId: nil,
	})

	return nil
}

func (m *MemoryStorage) BeginMatrixTransaction(ctx context.Context, destination string) (*storage.MatrixTransaction, storage.Transaction, error) {
	assert.NotNil(m.t, ctx, "context is required")

	// We are replicating the postgresql behaviour here, so we lock the entire "destinations" table even if there's more
	// EDUs we could return. We also need to match our own interface spec.

	if m.destinationLocks[destination] == nil {
		return nil, nil, sql.ErrNoRows
	}
	m.destinationLocks[destination].Lock() // unlocked in transaction

	// Get (and assign) 100 or fewer EDUs to a transaction
	txnId := storage.NextId()
	edus := make([]*storage.StoredEdu, 0)
	for _, edu := range m.destinationEdus[destination] {
		if len(edus) == 100 {
			break
		}
		edus = append(edus, edu.StoredEdu)
		edu.transactionId = internal.Pointer(txnId)
	}
	if len(edus) == 0 {
		m.destinationLocks[destination].Unlock()
		return nil, nil, sql.ErrNoRows
	}

	unwrappedEdus := make([]gomatrixserverlib.EDU, len(edus))
	for i, edu := range edus {
		unwrappedEdus[i] = edu.Payload
	}

	mxTxn := &storage.MatrixTransaction{
		TransactionId: txnId,
		Destination:   destination,
		Edus:          unwrappedEdus,
	}
	sqlTxn := &memoryDestinationTransaction{
		storage:       m,
		destination:   destination,
		transactionId: txnId,
	}
	return mxTxn, sqlTxn, nil
}

// mustClone - clones structs for reuse elsewhere. This does a relatively shallow clone using primitives.
// See implementation for details.
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

type memoryStateLearnTransaction struct { // Implements storage.Transaction
	storage *MemoryStorage
	row     *storage.StateLearnQueueItem
}

func (t *memoryStateLearnTransaction) Commit() error {
	newQueue := make([]*storage.StateLearnQueueItem, 0)
	for _, item := range t.storage.pendingLearnStateQueue {
		if item != t.row {
			newQueue = append(newQueue, item)
		}
	}
	t.storage.pendingLearnStateQueue = newQueue
	return nil
}

func (t *memoryStateLearnTransaction) Rollback() error {
	t.storage.learnStateQueue = append(t.storage.learnStateQueue, t.row)
	return t.Commit() // we're cheating a bit to avoid code duplication
}

type memoryDestinationTransaction struct {
	storage       *MemoryStorage
	destination   string
	transactionId string
	committed     bool
}

func (t *memoryDestinationTransaction) Commit() error {
	// Remove any "processed" EDUs before unlocking
	newEdus := make([]*memoryDestinationEdu, 0)
	for _, edu := range t.storage.destinationEdus[t.destination] {
		if edu.transactionId == nil || *edu.transactionId != t.transactionId {
			newEdus = append(newEdus, edu)
		}
	}
	t.storage.destinationEdus[t.destination] = newEdus
	t.storage.destinationLocks[t.destination].Unlock()
	t.committed = true
	return nil
}

func (t *memoryDestinationTransaction) Rollback() error {
	if t.committed {
		return nil
	}

	// Revert EDUs to "no assigned transaction" before unlocking
	for _, edu := range t.storage.destinationEdus[t.destination] {
		if edu.transactionId != nil && *edu.transactionId == t.transactionId {
			edu.transactionId = nil
		}
	}
	t.storage.destinationLocks[t.destination].Unlock()
	return nil
}
