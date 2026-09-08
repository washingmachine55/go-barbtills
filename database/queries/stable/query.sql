-- ============================================================
-- Task resolution. A task is addressed by name (case-insensitive,
-- the primary human handle), by seq (short permanent number), or
-- by uuid (scripting only).
-- ============================================================

-- name: GetTaskByName :one
SELECT id, name, status, type, priority, tags, category, seq, is_paused
FROM tasks
WHERE lower(name) = lower(sqlc.arg(name));

-- name: GetTaskBySeq :one
SELECT id, name, status, type, priority, tags, category, seq, is_paused
FROM tasks
WHERE seq = sqlc.arg(seq);

-- name: GetTaskByID :one
SELECT id, name, status, type, priority, tags, category, seq, is_paused
FROM tasks
WHERE id = sqlc.arg(id);

-- ============================================================
-- Tasks: write
-- ============================================================

-- name: CreateTask :one
INSERT INTO tasks (name, priority, category, type, tags)
VALUES (sqlc.arg(name), sqlc.arg(priority), sqlc.arg(category), sqlc.arg(type), sqlc.arg(tags))
RETURNING id, name, status, type, priority, tags, category, seq, is_paused;

-- name: RenameTask :one
UPDATE tasks SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id)
RETURNING id, name, status, type, priority, tags, category, seq, is_paused;

-- name: UpdateTaskMeta :one
UPDATE tasks
SET priority = COALESCE(sqlc.narg(priority), priority),
    type     = COALESCE(sqlc.narg(type),     type),
    tags     = COALESCE(sqlc.narg(tags),     tags),
    category = COALESCE(sqlc.narg(category), category)
WHERE id = sqlc.arg(id)
RETURNING id, name, status, type, priority, tags, category, seq, is_paused;

-- name: SetTaskStatus :one
UPDATE tasks SET status = sqlc.arg(status)
WHERE id = sqlc.arg(id)
RETURNING id, name, status, type, priority, tags, category, seq, is_paused;

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = sqlc.arg(id);

-- ============================================================
-- Tasks: the one list query. Serves list, status filter, name
-- search and single-task detail.
--
-- LEFT JOIN so a task with no sessions still appears.
-- Overall task time is the SUM of session durations, never
-- MAX(end)-MIN(start): that span only equals the sum when a task
-- has exactly one session, which pausing makes impossible.
-- closed_seconds covers finished sessions only; the live portion
-- of the open one is added client-side from open_started_at, so
-- the TUI can tick every second without querying.
--
-- Every possibly-NULL output uses CASE WHEN <guard> THEN NULL so
-- sqlc emits sql.NullTime / sql.NullInt64 rather than a bare
-- time.Time that panics on scan.
-- ============================================================

-- name: ListTasks :many
SELECT
    t.id, t.seq, t.name, t.status, t.is_paused, t.type, t.priority, t.tags, t.category,
    COUNT(s.id) FILTER (WHERE s.archived_at IS NULL) AS session_count,
    COUNT(s.id) FILTER (WHERE s.archived_at IS NOT NULL) AS archived_session_count,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (s.end_time - s.start_time)))
            FILTER (WHERE s.end_time IS NOT NULL AND s.archived_at IS NULL),
        0
    )::bigint AS closed_seconds,
    COALESCE(
        SUM(EXTRACT(EPOCH FROM (s.end_time - s.start_time)))
            FILTER (WHERE s.archived_at IS NOT NULL),
        0
    )::bigint AS archived_seconds,
    CASE WHEN COUNT(s.id) FILTER (WHERE s.end_time IS NULL) = 0
         THEN NULL ELSE MAX(s.start_time) FILTER (WHERE s.end_time IS NULL)
    END AS open_started_at,
    CASE WHEN COUNT(s.id) FILTER (WHERE s.end_time IS NULL) = 0
         THEN NULL ELSE MAX(s.seq) FILTER (WHERE s.end_time IS NULL)
    END AS open_session_seq,
    CASE WHEN COUNT(s.id) FILTER (WHERE s.archived_at IS NULL) = 0
         THEN NULL ELSE MIN(s.start_time) FILTER (WHERE s.archived_at IS NULL)
    END AS first_started_at,
    CASE WHEN COUNT(s.id) FILTER (WHERE s.end_time IS NULL) > 0
         THEN NULL ELSE MAX(s.end_time) FILTER (WHERE s.archived_at IS NULL)
    END AS last_ended_at
FROM tasks t
LEFT JOIN tasks_sessions s ON s.task_id = t.id
WHERE (sqlc.narg(seq)::bigint IS NULL OR t.seq = sqlc.narg(seq)::bigint)
  AND (sqlc.narg(status)::tasks_statuses IS NULL OR t.status = sqlc.narg(status)::tasks_statuses)
  AND (sqlc.narg(name_like)::text IS NULL OR t.name ILIKE '%' || sqlc.narg(name_like)::text || '%')
GROUP BY t.id
HAVING (sqlc.narg(only_open)::boolean IS NOT TRUE
        OR COUNT(s.id) FILTER (WHERE s.end_time IS NULL) > 0)
ORDER BY t.seq;

-- ============================================================
-- Sessions
-- ============================================================

-- name: OpenSession :one
INSERT INTO tasks_sessions (task_id) VALUES (sqlc.arg(task_id))
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: GetOpenSessionForTask :one
SELECT id, task_id, start_time, end_time, seq, archived_at
FROM tasks_sessions
WHERE task_id = sqlc.arg(task_id) AND end_time IS NULL;

-- name: CloseOpenSessionForTask :one
UPDATE tasks_sessions SET end_time = now()
WHERE task_id = sqlc.arg(task_id) AND end_time IS NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: GetSessionBySeq :one
SELECT id, task_id, start_time, end_time, seq, archived_at
FROM tasks_sessions
WHERE seq = sqlc.arg(seq);

-- name: CloseSessionBySeq :one
UPDATE tasks_sessions SET end_time = now()
WHERE seq = sqlc.arg(seq) AND end_time IS NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: SetSessionTimes :one
UPDATE tasks_sessions
SET start_time = COALESCE(sqlc.narg(start_time), start_time),
    end_time   = CASE WHEN sqlc.arg(clear_end_time)::boolean THEN NULL
                      ELSE COALESCE(sqlc.narg(end_time), end_time) END
WHERE seq = sqlc.arg(seq)
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: DeleteSessionBySeq :execrows
DELETE FROM tasks_sessions WHERE seq = sqlc.arg(seq);

-- name: ListSessionsForTask :many
SELECT s.id, s.seq, s.task_id, s.start_time, s.end_time, s.archived_at,
       (s.end_time IS NULL) AS is_open,
       EXTRACT(EPOCH FROM (COALESCE(s.end_time, now()) - s.start_time))::bigint AS duration_seconds
FROM tasks_sessions s
WHERE s.task_id = sqlc.arg(task_id)
  AND (sqlc.narg(include_archived)::boolean IS TRUE OR s.archived_at IS NULL)
ORDER BY s.start_time DESC, s.seq DESC;

-- name: TaskDuration :one
SELECT
    COUNT(*) FILTER (WHERE s.archived_at IS NULL) AS session_count,
    COUNT(*) FILTER (WHERE s.end_time IS NULL) AS open_session_count,
    COUNT(*) FILTER (WHERE s.archived_at IS NOT NULL) AS archived_session_count,
    COALESCE(SUM(EXTRACT(EPOCH FROM (s.end_time - s.start_time)))
             FILTER (WHERE s.end_time IS NOT NULL AND s.archived_at IS NULL), 0)::bigint AS closed_seconds,
    COALESCE(SUM(EXTRACT(EPOCH FROM (now() - s.start_time)))
             FILTER (WHERE s.end_time IS NULL), 0)::bigint AS open_seconds,
    COALESCE(SUM(EXTRACT(EPOCH FROM (s.end_time - s.start_time)))
             FILTER (WHERE s.archived_at IS NOT NULL), 0)::bigint AS archived_seconds
FROM tasks_sessions s
WHERE s.task_id = sqlc.arg(task_id);

-- name: ArchiveTaskSessions :many
-- Bank every finished session of a task, resetting its running total. A session
-- still open is deliberately left alone so a timer running across the rollover
-- keeps counting into the new period.
UPDATE tasks_sessions SET archived_at = now()
WHERE task_id = sqlc.arg(task_id) AND end_time IS NOT NULL AND archived_at IS NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: ArchiveSessionBySeq :one
UPDATE tasks_sessions SET archived_at = now()
WHERE seq = sqlc.arg(seq) AND end_time IS NOT NULL AND archived_at IS NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: UnarchiveSessionBySeq :one
UPDATE tasks_sessions SET archived_at = NULL
WHERE seq = sqlc.arg(seq) AND archived_at IS NOT NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- name: UnarchiveTaskSessions :many
-- Undo a rollover: bring every archived session of a task back into the total.
UPDATE tasks_sessions SET archived_at = NULL
WHERE task_id = sqlc.arg(task_id) AND archived_at IS NOT NULL
RETURNING id, task_id, start_time, end_time, seq, archived_at;

-- ============================================================
-- Admin
-- ============================================================

-- name: TruncateAllTaskData :exec
TRUNCATE TABLE tasks_sessions, tasks RESTART IDENTITY;

-- name: FindInconsistentTasks :many
-- Cross-row invariants a CHECK cannot express. Backs `tasks doctor`.
SELECT t.seq, t.name, t.status,
       COUNT(s.id) FILTER (WHERE s.end_time IS NULL) AS open_sessions
FROM tasks t
LEFT JOIN tasks_sessions s ON s.task_id = t.id
GROUP BY t.id
HAVING (t.status = 'In Progress' AND COUNT(s.id) FILTER (WHERE s.end_time IS NULL) = 0)
    OR (t.status <> 'In Progress' AND COUNT(s.id) FILTER (WHERE s.end_time IS NULL) > 0)
ORDER BY t.seq;
