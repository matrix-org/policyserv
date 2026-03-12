CREATE TABLE media_classifications (
    mxc_uri TEXT NOT NULL,
    community_id TEXT NOT NULL CONSTRAINT fk_media_classifications_community_id_communities_id REFERENCES communities(id),
    classifications JSONB NULL,
    PRIMARY KEY (mxc_uri, community_id)
);
