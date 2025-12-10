ALTER TABLE rooms DROP CONSTRAINT fk_rooms_community_id_communities_id;
ALTER TABLE rooms DROP COLUMN community_id;
DROP TABLE communities;
