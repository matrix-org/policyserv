ALTER TABLE communities ADD COLUMN api_access_token TEXT;
CREATE INDEX idx_communities_api_access_token ON communities (api_access_token);
