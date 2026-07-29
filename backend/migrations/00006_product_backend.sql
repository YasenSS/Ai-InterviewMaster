-- +goose Up
CREATE TABLE refresh_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    replaced_by_session_id uuid REFERENCES refresh_sessions(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_sessions_user_active_idx
    ON refresh_sessions (user_id, expires_at DESC)
    WHERE revoked_at IS NULL;
CREATE INDEX refresh_sessions_token_hash_idx ON refresh_sessions (token_hash);

ALTER TABLE question_sets
    ADD COLUMN updated_at timestamptz,
    ADD COLUMN source_question_set_id uuid REFERENCES question_sets(id) ON DELETE SET NULL;
UPDATE question_sets SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE question_sets
    ALTER COLUMN updated_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET NOT NULL;
CREATE INDEX question_sets_user_status_created_idx
    ON question_sets (user_id, status, created_at DESC, id DESC);
CREATE INDEX question_sets_resume_idx ON question_sets (user_id, resume_id, created_at DESC);

ALTER TABLE interview_sessions
    ADD COLUMN started_at timestamptz,
    ADD COLUMN duration_seconds integer NOT NULL DEFAULT 0,
    ADD COLUMN question_duration_seconds integer NOT NULL DEFAULT 180;

UPDATE interview_sessions
SET started_at = created_at
WHERE status IN ('active', 'completed');

UPDATE interview_sessions
SET duration_seconds = GREATEST(
        0,
        FLOOR(EXTRACT(EPOCH FROM (completed_at - created_at)))::integer
    )
WHERE status = 'completed'
  AND completed_at IS NOT NULL;

ALTER TABLE interview_sessions
    ADD CONSTRAINT interview_sessions_duration_nonnegative
        CHECK (duration_seconds >= 0),
    ADD CONSTRAINT interview_sessions_question_duration_supported
        CHECK (question_duration_seconds = 180);

CREATE INDEX interview_sessions_user_status_updated_idx
    ON interview_sessions (user_id, status, updated_at DESC, id DESC);

ALTER TABLE interview_turns
    ADD COLUMN started_at timestamptz,
    ADD COLUMN skipped_at timestamptz,
    ADD COLUMN time_spent_seconds integer NOT NULL DEFAULT 0;

UPDATE interview_turns
SET started_at = created_at,
    time_spent_seconds = CASE
        WHEN answered_at IS NULL THEN 0
        ELSE GREATEST(
            0,
            FLOOR(EXTRACT(EPOCH FROM (answered_at - created_at)))::integer
        )
    END
WHERE answered_at IS NOT NULL;

UPDATE interview_turns AS turn
SET started_at = session.started_at
FROM interview_sessions AS session
WHERE turn.session_id = session.id
  AND session.status = 'active'
  AND turn.ordinal = session.current_ordinal
  AND turn.started_at IS NULL;

ALTER TABLE interview_turns
    ADD CONSTRAINT interview_turns_time_spent_nonnegative
        CHECK (time_spent_seconds >= 0);

ALTER TABLE async_tasks
    ADD COLUMN retry_of_task_id uuid REFERENCES async_tasks(id) ON DELETE SET NULL,
    ADD COLUMN error_code text,
    ADD COLUMN error_summary text,
    ADD COLUMN started_at timestamptz;

UPDATE async_tasks
SET error_code = 'TASK_EXECUTION_FAILED',
    error_summary = '任务执行失败，请稍后重试',
    completed_at = COALESCE(completed_at, updated_at)
WHERE status = 'failed';

UPDATE async_tasks
SET started_at = created_at
WHERE status IN ('running', 'succeeded', 'failed');

WITH ranked_active_tasks AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY ref_id, task_type
               ORDER BY created_at DESC, id DESC
           ) AS active_rank
    FROM async_tasks
    WHERE status IN ('pending', 'running')
)
UPDATE async_tasks AS task
SET status = 'failed',
    error_code = 'DUPLICATE_TASK_SUPERSEDED',
    error_summary = '任务已由更新的请求接管',
    completed_at = now(),
    updated_at = now()
FROM ranked_active_tasks AS ranked
WHERE task.id = ranked.id
  AND ranked.active_rank > 1;

CREATE INDEX async_tasks_user_status_created_idx
    ON async_tasks (user_id, status, created_at DESC, id DESC);
CREATE INDEX async_tasks_retry_of_idx ON async_tasks (retry_of_task_id)
    WHERE retry_of_task_id IS NOT NULL;
CREATE UNIQUE INDEX async_tasks_active_ref_unique_idx
    ON async_tasks (ref_id, task_type)
    WHERE status IN ('pending', 'running');
CREATE UNIQUE INDEX async_tasks_active_retry_unique_idx
    ON async_tasks (retry_of_task_id)
    WHERE retry_of_task_id IS NOT NULL
      AND status IN ('pending', 'running');

UPDATE app_schema_metadata
SET value = '{"name":"product-backend","version":6}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP INDEX IF EXISTS async_tasks_active_retry_unique_idx;
DROP INDEX IF EXISTS async_tasks_active_ref_unique_idx;
DROP INDEX IF EXISTS async_tasks_retry_of_idx;
DROP INDEX IF EXISTS async_tasks_user_status_created_idx;
ALTER TABLE async_tasks
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS error_summary,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS retry_of_task_id;

ALTER TABLE interview_turns
    DROP CONSTRAINT IF EXISTS interview_turns_time_spent_nonnegative,
    DROP COLUMN IF EXISTS time_spent_seconds,
    DROP COLUMN IF EXISTS skipped_at,
    DROP COLUMN IF EXISTS started_at;

DROP INDEX IF EXISTS interview_sessions_user_status_updated_idx;
ALTER TABLE interview_sessions
    DROP CONSTRAINT IF EXISTS interview_sessions_question_duration_supported,
    DROP CONSTRAINT IF EXISTS interview_sessions_duration_nonnegative,
    DROP COLUMN IF EXISTS question_duration_seconds,
    DROP COLUMN IF EXISTS duration_seconds,
    DROP COLUMN IF EXISTS started_at;

DROP INDEX IF EXISTS question_sets_resume_idx;
DROP INDEX IF EXISTS question_sets_user_status_created_idx;
ALTER TABLE question_sets
    DROP COLUMN IF EXISTS source_question_set_id,
    DROP COLUMN IF EXISTS updated_at;

DROP TABLE IF EXISTS refresh_sessions;

UPDATE app_schema_metadata
SET value = '{"name":"milestone-3-beta","version":5}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';
