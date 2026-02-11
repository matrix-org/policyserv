package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/DavidHuie/gomigrate"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/metrics/dbmetrics"
	_ "github.com/lib/pq"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/ryanuber/go-glob"
	"golang.org/x/sync/singleflight"
)

type PostgresStorageConnectionConfig struct {
	Uri          string
	MaxOpenConns int
	MaxIdleConns int
}

type PostgresStorageConfig struct {
	// Read/Write Database connection config
	RWDatabase *PostgresStorageConnectionConfig
	// Readonly Database connection config. If nil, the RW database will be used for RO operations
	RODatabase *PostgresStorageConnectionConfig
	// File path to the directory containing migrations
	MigrationsPath string
}

type PostgresStorage struct {
	db         *sql.DB
	readonlyDb *sql.DB

	learnStateGroup *singleflight.Group
	learnStateCache *cache.Cache[string, error]

	roomSelectAll                        *sql.Stmt
	roomSelect                           *sql.Stmt
	roomUpsert                           *sql.Stmt
	roomDelete                           *sql.Stmt
	eventResultSelect                    *sql.Stmt
	eventResultUpsert                    *sql.Stmt
	userIdsAndDisplayNamesByRoomIdSelect *sql.Stmt
	banRulesSelectForRoom                *sql.Stmt
	communityUpsert                      *sql.Stmt
	communitySelect                      *sql.Stmt
	stateLearnQueueInsert                *sql.Stmt
	trustDataSelect                      *sql.Stmt
	trustDataUpsert                      *sql.Stmt
	keywordTemplateSelect                *sql.Stmt
	keywordTemplateUpsert                *sql.Stmt

	//userIdsAndDisplayNamesByRoomIdUpsert *sql.Stmt // We do the upsert manually to enter a transaction instead
	//banRulesUpsertForRoom                *sql.Stmt // We do the upsert manually to enter a transaction instead
	//stateLearnQueueSelect                *sql.Stmt // We do the select/delete manually to enter a transaction instead
}

func NewPostgresStorage(config *PostgresStorageConfig) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", config.RWDatabase.Uri)
	if err != nil {
		return nil, errors.Join(errors.New("failed to open read/write database"), err)
	}
	db.SetMaxOpenConns(config.RWDatabase.MaxOpenConns)
	db.SetMaxIdleConns(config.RWDatabase.MaxIdleConns)

	readonlyDb := db
	if config.RODatabase != nil {
		readonlyDb, err = sql.Open("postgres", config.RODatabase.Uri)
		if err != nil {
			errors.Join(errors.New("failed to open read-only database"), err)
		}
		readonlyDb.SetMaxOpenConns(config.RODatabase.MaxOpenConns)
		readonlyDb.SetMaxIdleConns(config.RODatabase.MaxIdleConns)
	}

	s := &PostgresStorage{
		db:              db,
		readonlyDb:      readonlyDb,
		learnStateGroup: new(singleflight.Group),
		learnStateCache: cache.New[string, error](cache.WithJanitorInterval[string, error](1 * time.Minute)),
	}
	if err = s.prepare(config.MigrationsPath); err != nil {
		return nil, errors.Join(fmt.Errorf("failed to run migrations with path '%s'", config.MigrationsPath), err)
	}
	return s, nil
}

func (s *PostgresStorage) prepare(migrationsDir string) error {
	// Migrate first
	if migrator, err := gomigrate.NewMigratorWithLogger(s.db, gomigrate.Postgres{}, migrationsDir, log.Default()); err != nil {
		return err
	} else {
		if err = migrator.Migrate(); err != nil {
			return err
		}
	}

	// Now set up all the prepared statements
	var err error
	if s.roomSelectAll, err = s.readonlyDb.Prepare("SELECT room_id, room_version, moderator_user_id, last_state_update_ts, community_id FROM rooms"); err != nil {
		return err
	}
	if s.roomSelect, err = s.readonlyDb.Prepare("SELECT room_id, room_version, moderator_user_id, last_state_update_ts, community_id FROM rooms WHERE room_id = $1"); err != nil {
		return err
	}
	if s.roomUpsert, err = s.db.Prepare("INSERT INTO rooms (room_id, room_version, moderator_user_id, last_state_update_ts, community_id) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (room_id) DO UPDATE SET room_version = $2, moderator_user_id = $3, last_state_update_ts = $4, community_id = $5;"); err != nil {
		return err
	}
	if s.roomDelete, err = s.db.Prepare("DELETE FROM rooms WHERE room_id = $1;"); err != nil {
		return err
	}
	if s.eventResultSelect, err = s.readonlyDb.Prepare("SELECT event_id, is_probably_spam, confidence_vectors FROM events WHERE event_id = $1"); err != nil {
		return err
	}
	if s.eventResultUpsert, err = s.db.Prepare("INSERT INTO events (event_id, is_probably_spam, confidence_vectors) VALUES ($1, $2, $3) ON CONFLICT (event_id) DO UPDATE SET is_probably_spam = $2, confidence_vectors = $3;"); err != nil {
		return err
	}
	if s.userIdsAndDisplayNamesByRoomIdSelect, err = s.readonlyDb.Prepare("SELECT user_id, displayname FROM displaynames WHERE room_id = $1"); err != nil {
		return err
	}
	if s.banRulesSelectForRoom, err = s.readonlyDb.Prepare("SELECT entity_type, entity_id FROM ban_rules WHERE room_id = $1;"); err != nil {
		return err
	}
	if s.communityUpsert, err = s.db.Prepare("INSERT INTO communities (id, name, config, api_access_token) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name = $2, config = $3, api_access_token = $4;"); err != nil {
		return err
	}
	if s.communitySelect, err = s.readonlyDb.Prepare("SELECT id, name, config, api_access_token FROM communities WHERE id = $1"); err != nil {
		return err
	}
	if s.stateLearnQueueInsert, err = s.db.Prepare("INSERT INTO state_learn_queue (room_id, at_event_id, via, after_ts) VALUES ($1, $2, $3, $4) ON CONFLICT (room_id) DO NOTHING;"); err != nil {
		return err
	}
	if s.trustDataSelect, err = s.readonlyDb.Prepare("SELECT data FROM trust_data WHERE source_name = $1 AND key = $2;"); err != nil {
		return err
	}
	if s.trustDataUpsert, err = s.db.Prepare("INSERT INTO trust_data (source_name, key, data) VALUES ($1, $2, $3) ON CONFLICT (source_name, key) DO UPDATE SET data = $3;"); err != nil {
		return err
	}
	if s.keywordTemplateSelect, err = s.readonlyDb.Prepare("SELECT name, body FROM keyword_templates WHERE name = $1;"); err != nil {
		return err
	}
	if s.keywordTemplateUpsert, err = s.db.Prepare("INSERT INTO keyword_templates (name, body) VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET body = $2;"); err != nil {
		return err
	}

	return nil
}

func (s *PostgresStorage) SendNotify(ctx context.Context, channel string, msg string) error {
	t := dbmetrics.StartSelfDatabaseTimer("SendNotify")
	defer t.ObserveDuration()
	_, err := s.db.ExecContext(ctx, "SELECT pg_notify($1, $2);", channel, msg)
	return err
}

func (s *PostgresStorage) Close() error {
	if err := s.db.Close(); err != nil {
		return err
	}
	if s.readonlyDb != nil {
		if err := s.readonlyDb.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStorage) GetAllRooms(ctx context.Context) ([]*StoredRoom, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetAllRooms")
	defer t.ObserveDuration()

	rows, err := s.roomSelectAll.QueryContext(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*StoredRoom, 0), nil
		}
		return nil, err
	}
	var rooms []*StoredRoom
	for rows.Next() {
		room := &StoredRoom{}
		err = rows.Scan(&room.RoomId, &room.RoomVersion, &room.ModeratorUserId, &room.LastCachedStateTimestampMillis, &room.CommunityId)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	return rooms, nil
}

func (s *PostgresStorage) GetRoom(ctx context.Context, roomId string) (*StoredRoom, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetRoom")
	defer t.ObserveDuration()

	room := &StoredRoom{}
	if err := s.roomSelect.QueryRowContext(ctx, roomId).Scan(&room.RoomId, &room.RoomVersion, &room.ModeratorUserId, &room.LastCachedStateTimestampMillis, &room.CommunityId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return room, nil
}

func (s *PostgresStorage) UpsertRoom(ctx context.Context, room *StoredRoom) error {
	t := dbmetrics.StartSelfDatabaseTimer("UpsertRoom")
	defer t.ObserveDuration()

	// Note: due to the `ps_room_community_change` trigger, we don't need to `NOTIFY policyserv_room_community_id_changed` here when the community ID changes.

	_, err := s.roomUpsert.ExecContext(ctx, room.RoomId, room.RoomVersion, room.ModeratorUserId, room.LastCachedStateTimestampMillis, room.CommunityId)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStorage) DeleteRoom(ctx context.Context, roomId string) error {
	t := dbmetrics.StartSelfDatabaseTimer("DeleteRoom")
	defer t.ObserveDuration()

	_, err := s.roomDelete.ExecContext(ctx, roomId)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStorage) GetEventResult(ctx context.Context, eventId string) (*StoredEventResult, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetEventResult")
	defer t.ObserveDuration()

	eventResult := &StoredEventResult{}
	var encodedVectors string
	if err := s.eventResultSelect.QueryRowContext(ctx, eventId).Scan(&eventResult.EventId, &eventResult.IsProbablySpam, &encodedVectors); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if encodedVectors != "" {
		if err := json.Unmarshal([]byte(encodedVectors), &eventResult.ConfidenceVectors); err != nil {
			return nil, err
		}
	} else {
		eventResult.ConfidenceVectors = confidence.NewConfidenceVectors() // populate empty
	}

	return eventResult, nil
}

func (s *PostgresStorage) UpsertEventResult(ctx context.Context, event *StoredEventResult) error {
	t := dbmetrics.StartSelfDatabaseTimer("UpsertEventResult")
	defer t.ObserveDuration()

	encodedVectors, err := json.Marshal(event.ConfidenceVectors)
	if err != nil {
		return err
	}

	_, err = s.eventResultUpsert.ExecContext(ctx, event.EventId, event.IsProbablySpam, string(encodedVectors))
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStorage) GetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string) ([]string, []string, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetUserIdsAndDisplayNamesByRoomId")
	defer t.ObserveDuration()

	rows, err := s.userIdsAndDisplayNamesByRoomIdSelect.QueryContext(ctx, roomId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]string, 0), make([]string, 0), nil
		}
		return nil, nil, err
	}

	userIds := &identifierSet{}
	displayNames := &identifierSet{}
	for rows.Next() {
		var userId string
		var displayName string
		if err = rows.Scan(&userId, &displayName); err != nil {
			return nil, nil, err
		}
		userIds.Add(userId)
		displayNames.Add(displayName)
	}

	return userIds.ToSlice(), displayNames.ToSlice(), nil
}

func (s *PostgresStorage) SetUserIdsAndDisplayNamesByRoomId(ctx context.Context, roomId string, userIds []string, displayNames []string) error {
	t := dbmetrics.StartSelfDatabaseTimer("SetUserIdsAndDisplayNamesByRoomId")
	defer t.ObserveDuration()

	txn, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if _, err = txn.Exec("DELETE FROM displaynames WHERE room_id = $1;", roomId); err != nil {
		return err
	}
	for i, userId := range userIds {
		if _, err = txn.Exec("INSERT INTO displaynames (room_id, user_id, displayname) VALUES ($1, $2, $3);", roomId, userId, displayNames[i]); err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *PostgresStorage) IsUserBannedInList(ctx context.Context, listRoomId string, userId string) (bool, error) {
	t := dbmetrics.StartSelfDatabaseTimer("IsUserBannedInList")
	defer t.ObserveDuration()

	parsedUserId, err := spec.NewUserID(userId, true)
	if err != nil {
		return false, err
	}

	rows, err := s.banRulesSelectForRoom.QueryContext(ctx, listRoomId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	for rows.Next() {
		var entityType string
		var entityId string
		if err = rows.Scan(&entityType, &entityId); err != nil {
			return false, err
		}
		entity := parsedUserId.String()
		if entityType == "m.policy.rule.server" {
			entity = string(parsedUserId.Domain())
		}
		if glob.Glob(entityId, entity) {
			log.Println("User", userId, "is banned via list", listRoomId, "with rule", entityId, " (entity type:", entityType, ")")
			return true, nil
		}
	}

	return false, nil
}

func (s *PostgresStorage) SetListBanRules(ctx context.Context, listRoomId string, entityToEntityType map[string]string) error {
	t := dbmetrics.StartSelfDatabaseTimer("SetListBanRules")
	defer t.ObserveDuration()

	txn, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if _, err = txn.Exec("DELETE FROM ban_rules WHERE room_id = $1;", listRoomId); err != nil {
		return err
	}
	for entity, entityType := range entityToEntityType {
		if _, err = txn.Exec("INSERT INTO ban_rules (room_id, entity_type, entity_id) VALUES ($1, $2, $3);", listRoomId, entityType, entity); err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *PostgresStorage) CreateCommunity(ctx context.Context, name string) (*StoredCommunity, error) {
	t := dbmetrics.StartSelfDatabaseTimer("CreateCommunity")
	defer t.ObserveDuration()

	community := &StoredCommunity{
		CommunityId: NextId(),
		Name:        name,
		Config:      &config.CommunityConfig{}, // empty by default
	}
	_, err := s.communityUpsert.ExecContext(ctx, community.CommunityId, community.Name, community.Config)
	if err != nil {
		return nil, err
	}
	return community, nil
}

func (s *PostgresStorage) UpsertCommunity(ctx context.Context, community *StoredCommunity) error {
	t := dbmetrics.StartSelfDatabaseTimer("UpsertCommunity")
	defer t.ObserveDuration()

	// Note: due to the `ps_community_config_change` trigger, we don't need to `NOTIFY policyserv_community_config_changed` here.

	_, err := s.communityUpsert.ExecContext(ctx, community.CommunityId, community.Name, community.Config, community.ApiAccessToken)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStorage) GetCommunity(ctx context.Context, communityId string) (*StoredCommunity, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetCommunity")
	defer t.ObserveDuration()

	community := &StoredCommunity{}
	if err := s.communitySelect.QueryRowContext(ctx, communityId).Scan(&community.CommunityId, &community.Name, &community.Config, &community.ApiAccessToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return community, nil
}

func (s *PostgresStorage) PushStateLearnQueue(ctx context.Context, item *StateLearnQueueItem) error {
	t := dbmetrics.StartSelfDatabaseTimer("PushStateLearnQueue")
	defer t.ObserveDuration()

	val, ok := s.learnStateCache.Get(item.RoomId)
	if ok {
		if val == nil {
			return nil
		}
		return val.(error)
	}

	_, err, _ := s.learnStateGroup.Do(item.RoomId, func() (interface{}, error) {
		_, err := s.stateLearnQueueInsert.ExecContext(ctx, item.RoomId, item.AtEventId, item.ViaServer, item.AfterTimestampMillis)
		s.learnStateCache.Set(item.RoomId, err, cache.WithExpiration(1*time.Minute))
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	return err
}

func (s *PostgresStorage) PopStateLearnQueue(ctx context.Context) (*StateLearnQueueItem, Transaction, error) {
	t := dbmetrics.StartSelfDatabaseTimer("PopStateLearnQueue")
	defer t.ObserveDuration()

	txn, err := s.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	// "FOR UPDATE SKIP LOCKED" prevents postgres from returning rows which are locked in another transaction.
	// This is why we start a transaction that tries to delete the row - this places a lock on the row.
	// Note: we do the select and delete as a single operation to avoid a situation where another process takes
	// a lock out on the same row as us.
	r := txn.QueryRowContext(ctx, "DELETE FROM state_learn_queue WHERE room_id IN (SELECT s.room_id FROM state_learn_queue AS s WHERE after_ts <= (EXTRACT(EPOCH FROM NOW()) * 1000) LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING room_id, at_event_id, via, after_ts;")
	val := &StateLearnQueueItem{}
	if err = r.Scan(&val.RoomId, &val.AtEventId, &val.ViaServer, &val.AfterTimestampMillis); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			defer txn.Rollback()
			return nil, nil, nil
		}
		return nil, nil, err
	}

	return val, txn, nil
}

func (s *PostgresStorage) GetTrustData(ctx context.Context, sourceName string, key string, data any) error {
	t := dbmetrics.StartSelfDatabaseTimer("GetTrustData")
	defer t.ObserveDuration()

	r := s.trustDataSelect.QueryRowContext(ctx, sourceName, key)
	b := make([]byte, 0)
	err := r.Scan(&b)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, data)
}

func (s *PostgresStorage) SetTrustData(ctx context.Context, sourceName string, key string, data any) error {
	t := dbmetrics.StartSelfDatabaseTimer("SetTrustData")
	defer t.ObserveDuration()

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = s.trustDataUpsert.ExecContext(ctx, sourceName, key, b)
	return err
}

func (s *PostgresStorage) UpsertKeywordTemplate(ctx context.Context, template *StoredKeywordTemplate) error {
	t := dbmetrics.StartSelfDatabaseTimer("UpsertKeywordTemplate")
	defer t.ObserveDuration()

	_, err := s.keywordTemplateUpsert.ExecContext(ctx, template.Name, template.Body)
	return err
}

func (s *PostgresStorage) GetKeywordTemplate(ctx context.Context, name string) (*StoredKeywordTemplate, error) {
	t := dbmetrics.StartSelfDatabaseTimer("GetKeywordTemplate")
	defer t.ObserveDuration()

	r := s.keywordTemplateSelect.QueryRowContext(ctx, name)
	val := &StoredKeywordTemplate{}
	err := r.Scan(&val.Name, &val.Body)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Deduplicates strings given to it
type identifierSet struct {
	identifiers map[string]bool
}

func (s *identifierSet) Add(identifiers ...string) *identifierSet {
	if s.identifiers == nil {
		s.identifiers = make(map[string]bool)
	}
	for _, identifier := range identifiers {
		s.identifiers[identifier] = true
	}
	return s
}

func (s *identifierSet) ToSlice() []string {
	if s.identifiers == nil {
		return make([]string, 0)
	}
	identifiers := make([]string, 0, len(s.identifiers))
	for identifier := range s.identifiers {
		identifiers = append(identifiers, identifier)
	}
	return identifiers
}
