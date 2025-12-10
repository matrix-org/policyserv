ALTER TABLE rooms ADD COLUMN last_state_update_ts BIGINT NOT NULL DEFAULT 0;
CREATE TABLE displaynames (
    room_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    displayname TEXT NOT NULL,
    PRIMARY KEY (room_id, user_id)
);
CREATE INDEX displaynames_room_id ON displaynames (room_id);
CREATE TABLE ban_rules (
    room_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    PRIMARY KEY (room_id, entity_type, entity_id)
);
CREATE INDEX ban_rules_room_id ON ban_rules (room_id);