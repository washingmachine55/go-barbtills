-- +goose Up

-- Archiving a session takes it out of the task's running total while keeping it
-- in the record. A recurring task can then be rolled over -- at the end of a day
-- or a billing period -- so the next period starts from zero without losing any
-- history.
ALTER TABLE tasks_sessions
	ADD COLUMN archived_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- An open session has no duration to bank yet, so it cannot be archived.
ALTER TABLE tasks_sessions
	ADD CONSTRAINT tasks_sessions_archive_requires_end
	CHECK (archived_at IS NULL OR end_time IS NOT NULL);

-- Every total and session listing filters on this, so index the live rows.
CREATE INDEX tasks_sessions_task_id_active_idx
	ON tasks_sessions (task_id)
	WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS tasks_sessions_task_id_active_idx;
ALTER TABLE tasks_sessions
	DROP CONSTRAINT IF EXISTS tasks_sessions_archive_requires_end;
ALTER TABLE tasks_sessions DROP COLUMN archived_at;
