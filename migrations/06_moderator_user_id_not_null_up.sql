UPDATE rooms SET moderator_user_id = '' WHERE moderator_user_id IS NULL;
ALTER TABLE rooms ALTER COLUMN moderator_user_id SET NOT NULL;
