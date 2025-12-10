CREATE OR REPLACE FUNCTION notify_community_config_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the config column has changed
    IF NEW.config IS DISTINCT FROM OLD.config THEN
        PERFORM pg_notify('policyserv_community_config_changed', NEW.id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ps_community_config_change AFTER UPDATE OF config ON communities FOR EACH ROW EXECUTE FUNCTION notify_community_config_change();

CREATE OR REPLACE FUNCTION notify_room_community_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the community ID column has changed
    IF NEW.community_id IS DISTINCT FROM OLD.community_id THEN
        PERFORM pg_notify('policyserv_room_community_id_changed', NEW.room_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ps_room_community_change AFTER UPDATE OF community_id ON rooms FOR EACH ROW EXECUTE FUNCTION notify_room_community_change();
