-- We're using an attribute-based permission system, so we'd just keep adding `can_do_x` columns for future permissions.
ALTER TABLE communities ADD COLUMN can_self_join_rooms BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN communities.can_self_join_rooms IS 'Whether the community can use the "Join Rooms" API to associate themselves with new rooms.';
