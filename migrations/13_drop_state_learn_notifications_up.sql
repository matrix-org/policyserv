-- This is essentially a partial rollback of migration 09.

DROP TRIGGER ps_state_learn_insert ON state_learn_queue;
DROP FUNCTION notify_state_learn_queue;
