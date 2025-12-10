-- Bring the old columns back as NULL
ALTER TABLE events ADD COLUMN reasons_csv TEXT;
ALTER TABLE events ADD COLUMN codes_csv TEXT;

-- Populate those fields
UPDATE events SET codes_csv = 'spam,ok\n' WHERE is_probably_spam = TRUE;
UPDATE events SET codes_csv = 'ok\n' WHERE is_probably_spam = FALSE;
UPDATE events SET reasons_csv = COALESCE(
    (
        -- Grab each key and CSV it, with trailing newline for compatibility
        SELECT string_agg(key, ',' ORDER BY key) || E'\n'
        FROM jsonb_each_text(confidence_vectors::jsonb)
    ),
    E'\n' -- default to empty csv
);

-- Drop new columns
ALTER TABLE events DROP COLUMN confidence_vectors;
ALTER TABLE events DROP COLUMN is_probably_spam;

-- Return old columns to NOT NULL
ALTER TABLE events ALTER COLUMN codes_csv SET NOT NULL;
ALTER TABLE events ALTER COLUMN reasons_csv SET NOT NULL;