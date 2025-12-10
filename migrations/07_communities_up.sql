CREATE TABLE communities (id TEXT NOT NULL PRIMARY KEY, name TEXT NOT NULL, config JSONB NOT NULL);
COMMENT ON COLUMN communities.id IS 'We use non-linear IDs to ensure callers cannot find out how many communities we have.';
INSERT INTO communities VALUES ('default', 'default', '{}');
ALTER TABLE rooms ADD COLUMN community_id TEXT DEFAULT 'default';
ALTER TABLE rooms ADD CONSTRAINT fk_rooms_community_id_communities_id FOREIGN KEY (community_id) REFERENCES communities(id);
