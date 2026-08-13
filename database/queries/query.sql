-- name: CompletedTasks :many
SELECT id, task_name, start_time, end_time FROM tasks WHERE end_time IS NOT NULL ORDER BY id;

-- name: RunningTasks :many
SELECT id, task_name, start_time, end_time FROM tasks WHERE end_time IS NULL ORDER BY id;

-- name: ShowSpecificTimer :one
SELECT id, task_name, start_time, end_time FROM tasks WHERE id = $1;

-- name: CreateNewTask :one
INSERT INTO tasks (task_name, start_time) VALUES ($1, $2) RETURNING id, task_name, start_time, end_time;

-- name: UpdateSelectedTask :one
UPDATE tasks SET end_time = $1 WHERE id = $2 AND end_time IS null RETURNING id, task_name, start_time, end_time;

-- name: TruncateTasks :exec
TRUNCATE TABLE tasks RESTART IDENTITY CASCADE;
