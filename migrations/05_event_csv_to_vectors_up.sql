-- Start with NULL, then populate and make NOT NULL
ALTER TABLE events ADD COLUMN is_probably_spam BOOLEAN NULL;
ALTER TABLE events ADD COLUMN confidence_vectors TEXT NULL;

-- Populate new fields
UPDATE events SET is_probably_spam = TRUE WHERE codes_csv LIKE '%spam%' OR codes_csv LIKE '%unsure%' OR codes_csv LIKE '%reject%';
UPDATE events SET is_probably_spam = FALSE where is_probably_spam IS NULL;
UPDATE events
SET confidence_vectors = COALESCE(
    -- Convert CSV to a JSON object with values of `1.0` for each reason type
    -- "trim(both E' \n\r\t' FROM reason)" allows us to remove excess whitespace
    (SELECT jsonb_object_agg(trim(both E' \n\r\t' FROM reason), 1.0)
     FROM unnest(string_to_array(reasons_csv, ',')) AS reason
     WHERE trim(both E' \n\r\t' FROM reason) <> '')::text,
    '{}' -- empty JSON default
);

-- Drop old fields
ALTER TABLE events DROP COLUMN reasons_csv;
ALTER TABLE events DROP COLUMN codes_csv;

-- Make new fields NOT NULL
ALTER TABLE events ALTER COLUMN is_probably_spam SET NOT NULL;
ALTER TABLE events ALTER COLUMN confidence_vectors SET NOT NULL;
