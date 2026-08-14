-- +goose Up
-- +goose statementbegin
ALTER TABLE tasks
	ALTER COLUMN id DROP DEFAULT,
	ALTER COLUMN id TYPE UUID USING uuidv7(),
	ALTER COLUMN id SET DEFAULT uuidv7(),
	ADD COLUMN is_paused BOOLEAN NOT NULL DEFAULT FALSE,
	DROP COLUMN start_time,
	DROP COLUMN end_time;
-- +goose statementend

-- +goose statementbegin
CREATE TABLE tasks_sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    task_id UUID NOT NULL,
   	start_time TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	end_time TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    CONSTRAINT fk_task_session_parent
    	FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
-- +goose statementend

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
--
------------------------------------------------------------------------------------------------
--
-- +goose Down
-- +goose statementbegin
DROP TABLE tasks_sessions;
ALTER TABLE tasks
	DROP COLUMN id CASCADE;
-- +goose statementend
-- +goose statementbegin
ALTER TABLE tasks ADD COLUMN new_id INT GENERATED ALWAYS AS IDENTITY;
-- +goose statementend
-- +goose statementbegin
ALTER TABLE tasks
	ADD COLUMN start_time TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	ADD COLUMN end_time TIMESTAMP WITH TIME ZONE DEFAULT NULL;
-- +goose statementend
-- +goose statementbegin
ALTER TABLE tasks
	RENAME COLUMN new_id TO id;
-- +goose statementend
