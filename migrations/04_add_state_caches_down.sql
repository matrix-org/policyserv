ALTER TABLE rooms DROP COLUMN last_state_update_ts;
DROP INDEX displaynames_room_id;
DROP TABLE displaynames;
DROP INDEX ban_rules_room_id;
DROP TABLE ban_rules;
