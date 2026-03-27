CREATE TABLE destinations (
    destination TEXT NOT NULL PRIMARY KEY,
    last_transaction_id TEXT NOT NULL DEFAULT ''
);

CREATE TABLE destination_edus (
    id BIGSERIAL NOT NULL PRIMARY KEY, -- autoincrements, doesn't need to be set by an insert
    destination TEXT NOT NULL REFERENCES destinations(destination) ON DELETE CASCADE,
    edu JSONB NOT NULL
);
CREATE INDEX destination_edus_destination ON destination_edus (destination);

-- We also create a trigger to wake EDU sending code upon new EDUs being available
CREATE OR REPLACE FUNCTION notify_edu_inserted()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('policyserv_edu_for_destination', NEW.destination);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ps_edu_inserted AFTER INSERT ON destination_edus FOR EACH ROW EXECUTE PROCEDURE notify_edu_inserted();
