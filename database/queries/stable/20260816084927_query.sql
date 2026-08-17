-- name: CompletedTasks :many
SELECT t.id, t.name, MIN(s.start_time) AS start_time, MAX(s.end_time) AS end_time
FROM tasks t
JOIN tasks_sessions s ON s.task_id = t.id
GROUP BY t.id, t.name
HAVING COUNT(*) FILTER (WHERE s.end_time IS NULL) = 0
ORDER BY t.id;

-- name: RunningTasks :many
SELECT t.id, t.name, MIN(s.start_time) AS start_time, NULL::timestamptz AS end_time
FROM tasks t
JOIN tasks_sessions s ON s.task_id = t.id
GROUP BY t.id, t.name
HAVING COUNT(*) FILTER (WHERE s.end_time IS NULL) > 0
ORDER BY t.id;

-- name: ShowSpecificTimer :one
SELECT t.id, t.name, t.is_paused,
       MIN(s.start_time) AS start_time,
       CASE WHEN COUNT(*) FILTER (WHERE s.end_time IS NULL) > 0
            THEN NULL ELSE MAX(s.end_time) END AS end_time
FROM tasks t
LEFT JOIN tasks_sessions s ON s.task_id = t.id
WHERE t.id = $1
GROUP BY t.id, t.name, t.is_paused;

-- name: CreateNewTask :one
INSERT INTO tasks (name, priority, category, type, tags) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: StartTaskSession :one
INSERT INTO tasks_sessions (task_id, start_time) VALUES ($1, $2) RETURNING *;

-- name: UpdateSelectedTask :one
UPDATE tasks_sessions SET end_time = now()
WHERE task_id = $1 AND end_time IS NULL
RETURNING id, task_id, start_time, end_time;

-- name: TruncateTasks :exec
-- BEGIN
TRUNCATE TABLE tasks RESTART IDENTITY CASCADE;
TRUNCATE TABLE tasks_sessions RESTART IDENTITY CASCADE;
-- END;
