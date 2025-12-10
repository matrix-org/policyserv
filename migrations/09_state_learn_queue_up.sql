CREATE TABLE state_learn_queue (
    room_id TEXT PRIMARY KEY,
    at_event_id TEXT NOT NULL,
    via TEXT NOT NULL
);

CREATE OR REPLACE FUNCTION notify_state_learn_queue()
RETURNS TRIGGER AS $$
BEGIN
    -- Always notify
    PERFORM pg_notify('policyserv_new_state_to_learn', NEW.room_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ps_state_learn_insert AFTER INSERT ON state_learn_queue FOR EACH ROW EXECUTE FUNCTION notify_state_learn_queue();
