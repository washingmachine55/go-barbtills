-- +goose Up
-- +goose Statementbegin
CREATE TYPE tasks_statuses AS ENUM ('Pending', 'In Progress', 'Completed', 'Paused', 'Archived');
CREATE TYPE tasks_types AS ENUM ('Recurring', 'One Time');
CREATE TYPE tasks_priorities AS ENUM ('Unknown', 'Very Low', 'Low', 'Medium', 'High', 'Very High');
CREATE TYPE tasks_categories AS ENUM ('Unknown', 'Client Project', 'Personal Project', 'Troubleshooting', 'Routine');
-- +goose Statementend
-- +goose Statementbegin
ALTER TABLE tasks
	RENAME COLUMN task_name TO name;

ALTER TABLE tasks
	ADD COLUMN status tasks_statuses NOT NULL DEFAULT 'Pending';

ALTER TABLE tasks
	ADD COLUMN type tasks_types NOT NULL DEFAULT 'One Time';

ALTER TABLE tasks
	ADD COLUMN priority tasks_priorities NOT NULL DEFAULT 'Unknown';

ALTER TABLE tasks
	ADD COLUMN tags VARCHAR(55)[] NOT NULL DEFAULT '{}';

ALTER TABLE tasks
	ADD COLUMN category tasks_categories[] NOT NULL DEFAULT '{"Unknown"}';
-- +goose Statementend
-- +goose statementbegin
DROP FUNCTION IF EXISTS update_tasks_status;
DROP TRIGGER IF EXISTS update_tasks_is_paused;
CREATE FUNCTION update_tasks_status_to_completed() RETURNS TRIGGER AS $$
BEGIN
	UPDATE tasks t
	SET status = 'Completed'
	FROM new_task_paused_status n
	WHERE t.id = n.task_id AND n.end_time IS NOT NULL;
	RETURN NULL;
END;
$$ LANGUAGE plpgsql VOLATILE;
-- +goose statementend

-- +goose statementbegin
CREATE TRIGGER update_tasks_is_paused
	AFTER UPDATE ON tasks_sessions
	REFERENCING OLD TABLE AS old_task_paused_status NEW TABLE AS new_task_paused_status
	FOR EACH STATEMENT
	EXECUTE FUNCTION update_tasks_status();
-- +goose statementend


-- +goose Down
DROP TYPE IF EXISTS tasks_statuses, tasks_types, tasks_priorities, tasks_categories CASCADE;
ALTER TABLE tasks
	RENAME COLUMN name TO task_name;
ALTER TABLE tasks DROP COLUMN status, type, priority, tags, category;

-- +goose statementbegin
CREATE FUNCTION update_tasks_status() RETURNS TRIGGER AS $$
BEGIN
	UPDATE tasks t
	SET is_paused = (n.end_time IS NOT NULL)
	FROM new_task_paused_status n
	WHERE t.id = n.task_id;
	RETURN NULL;
END;
$$ LANGUAGE plpgsql VOLATILE;
-- +goose statementend

-- +goose statementbegin
CREATE TRIGGER update_tasks_is_paused
	AFTER UPDATE ON tasks_sessions
	REFERENCING OLD TABLE AS old_task_paused_status NEW TABLE AS new_task_paused_status
	FOR EACH STATEMENT
	EXECUTE FUNCTION update_tasks_status();
-- +goose statementend
