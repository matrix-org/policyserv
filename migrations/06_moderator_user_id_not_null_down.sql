ALTER TABLE rooms ALTER COLUMN moderator_user_id DROP NOT NULL;
UPDATE rooms SET moderator_user_id = NULL WHERE moderator_user_id = '';
