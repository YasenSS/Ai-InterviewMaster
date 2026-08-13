-- +goose Up
ALTER TYPE question_set_status ADD VALUE IF NOT EXISTS 'generating';
ALTER TYPE question_set_status ADD VALUE IF NOT EXISTS 'failed';
ALTER TYPE question_set_status ADD VALUE IF NOT EXISTS 'degraded';

CREATE TABLE model_invocations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id uuid REFERENCES async_tasks(id) ON DELETE SET NULL,
    session_id uuid REFERENCES interview_sessions(id) ON DELETE SET NULL,
    resource_type text NOT NULL DEFAULT 'unknown',
    resource_id uuid,
    provider text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    prompt_key text NOT NULL DEFAULT '',
    prompt_version text NOT NULL DEFAULT '',
    status text NOT NULL,
    attempt smallint NOT NULL DEFAULT 1,
    input_hash text,
    output_hash text,
    prompt_tokens integer NOT NULL DEFAULT 0,
    completion_tokens integer NOT NULL DEFAULT 0,
    total_tokens integer NOT NULL DEFAULT 0,
    estimated_cost_micros bigint NOT NULL DEFAULT 0,
    latency_ms integer NOT NULL DEFAULT 0,
    error_code text,
    trace_id text,
    request_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX model_invocations_user_created_idx ON model_invocations (user_id, created_at DESC);
CREATE INDEX model_invocations_resource_idx ON model_invocations (resource_id);
CREATE INDEX model_invocations_trace_idx ON model_invocations (trace_id);

ALTER TABLE question_sets
    ADD COLUMN IF NOT EXISTS blueprint jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS input_hash text,
    ADD COLUMN IF NOT EXISTS prompt_version text,
    ADD COLUMN IF NOT EXISTS model_name text;

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS capability_key text,
    ADD COLUMN IF NOT EXISTS difficulty text;

ALTER TABLE interview_sessions
    ADD COLUMN IF NOT EXISTS blueprint jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE interview_reports
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'completed',
    ADD COLUMN IF NOT EXISTS error_code text,
    ADD COLUMN IF NOT EXISTS error_summary text,
    ADD COLUMN IF NOT EXISTS degraded boolean NOT NULL DEFAULT false;

ALTER TABLE resume_versions
    ADD COLUMN IF NOT EXISTS extractor_model text,
    ADD COLUMN IF NOT EXISTS prompt_version text;

COMMENT ON TABLE model_invocations IS 'AI call audit. Query cost outliers with: SELECT user_id, date_trunc(''day'', created_at) AS day, sum(total_tokens), sum(estimated_cost_micros) FROM model_invocations GROUP BY 1, 2 ORDER BY 4 DESC;';

UPDATE app_schema_metadata
SET value = '{"name":"agent-p0","version":7}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
ALTER TABLE resume_versions
    DROP COLUMN IF EXISTS prompt_version,
    DROP COLUMN IF EXISTS extractor_model;
ALTER TABLE interview_reports
    DROP COLUMN IF EXISTS degraded,
    DROP COLUMN IF EXISTS error_summary,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS status;
ALTER TABLE interview_sessions DROP COLUMN IF EXISTS blueprint;
ALTER TABLE questions
    DROP COLUMN IF EXISTS difficulty,
    DROP COLUMN IF EXISTS capability_key;
ALTER TABLE question_sets
    DROP COLUMN IF EXISTS model_name,
    DROP COLUMN IF EXISTS prompt_version,
    DROP COLUMN IF EXISTS input_hash,
    DROP COLUMN IF EXISTS blueprint;
DROP TABLE IF EXISTS model_invocations;
UPDATE app_schema_metadata
SET value = '{"name":"product-backend","version":6}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';
