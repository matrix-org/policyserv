CREATE TABLE trust_data (
    source_name TEXT NOT NULL,
    key TEXT NOT NULL,
    data JSONB NOT NULL,
    PRIMARY KEY (source_name, key)
);
