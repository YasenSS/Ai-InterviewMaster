-- +goose Up
ALTER TABLE interview_sessions
    ADD COLUMN IF NOT EXISTS agent_mode text NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS phase text NOT NULL DEFAULT 'preparing',
    ADD COLUMN IF NOT EXISTS policy_version text NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS interviewer_prompt_version text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS min_turns integer NOT NULL DEFAULT 8,
    ADD COLUMN IF NOT EXISTS target_turns integer NOT NULL DEFAULT 12,
    ADD COLUMN IF NOT EXISTS max_turns integer NOT NULL DEFAULT 16,
    ADD COLUMN IF NOT EXISTS time_budget_minutes integer NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS max_follow_up_depth integer NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS max_follow_ups_total integer NOT NULL DEFAULT 4,
    ADD COLUMN IF NOT EXISTS capability_progress jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS completion_reason text,
    ADD COLUMN IF NOT EXISTS decision_version bigint NOT NULL DEFAULT 0;

UPDATE interview_sessions
SET phase = CASE status::text
    WHEN 'active' THEN 'answering'
    WHEN 'completed' THEN 'completed'
    WHEN 'abandoned' THEN 'completed'
    ELSE 'preparing'
END;

ALTER TABLE interview_sessions
    ADD CONSTRAINT interview_sessions_agent_mode_chk
        CHECK (agent_mode IN ('ai', 'rule', 'legacy')),
    ADD CONSTRAINT interview_sessions_phase_chk
        CHECK (phase IN ('preparing', 'answering', 'deciding', 'decision_failed', 'completed')),
    ADD CONSTRAINT interview_sessions_policy_bounds_chk
        CHECK (
            min_turns >= 1
            AND min_turns <= target_turns
            AND target_turns <= max_turns
            AND max_turns <= 40
            AND time_budget_minutes BETWEEN 5 AND 120
            AND max_follow_up_depth BETWEEN 0 AND 2
            AND max_follow_ups_total >= 0
            AND max_follow_ups_total < max_turns
        ),
    ADD CONSTRAINT interview_sessions_status_phase_chk
        CHECK (
            (status::text IN ('draft', 'preparing', 'failed') AND phase = 'preparing')
            OR (status::text = 'active' AND phase IN ('answering', 'deciding', 'decision_failed'))
            OR (status::text IN ('completed', 'abandoned') AND phase = 'completed')
        );

CREATE INDEX IF NOT EXISTS interview_sessions_user_phase_updated_idx
    ON interview_sessions (user_id, status, phase, updated_at DESC);

ALTER TABLE interview_turns
    ADD COLUMN IF NOT EXISTS intent text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expected_points jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS difficulty text NOT NULL DEFAULT 'medium',
    ADD COLUMN IF NOT EXISTS evidence_fact_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS decision_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS coverage_observation jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS generation_mode text NOT NULL DEFAULT 'legacy';

ALTER TABLE interview_turns
    ADD CONSTRAINT interview_turns_difficulty_v2_chk
        CHECK (difficulty IN ('easy', 'medium', 'hard')),
    ADD CONSTRAINT interview_turns_generation_mode_chk
        CHECK (generation_mode IN ('ai', 'fallback', 'legacy'));

ALTER TABLE interview_turns
    ADD CONSTRAINT interview_turns_invocation_id_fkey
        FOREIGN KEY (invocation_id) REFERENCES model_invocations(id) ON DELETE SET NULL;

CREATE TABLE interview_turn_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    answered_turn_id uuid NOT NULL REFERENCES interview_turns(id) ON DELETE CASCADE,
    next_turn_id uuid REFERENCES interview_turns(id) ON DELETE SET NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending',
    action text,
    attempt integer NOT NULL DEFAULT 0,
    input_hash text NOT NULL,
    model_invocation_id uuid REFERENCES model_invocations(id) ON DELETE SET NULL,
    error_code text,
    error_summary text,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (answered_turn_id),
    UNIQUE (next_turn_id),
    CONSTRAINT interview_turn_decisions_status_chk
        CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT interview_turn_decisions_action_chk
        CHECK (action IS NULL OR action IN ('deepen', 'switch_capability', 'recommend_finish')),
    CONSTRAINT interview_turn_decisions_attempt_chk CHECK (attempt >= 0)
);

CREATE INDEX interview_turn_decisions_pending_idx
    ON interview_turn_decisions (status, updated_at)
    WHERE status IN ('pending', 'running');
CREATE INDEX interview_turn_decisions_session_idx
    ON interview_turn_decisions (session_id, created_at DESC);
CREATE INDEX interview_turn_decisions_user_idx
    ON interview_turn_decisions (user_id, created_at DESC);

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS material_kind text NOT NULL DEFAULT 'legacy';
ALTER TABLE questions
    ADD CONSTRAINT questions_material_kind_chk
        CHECK (material_kind IN ('anchor', 'fallback', 'legacy'));

UPDATE app_schema_metadata
SET value = '{"name":"agent-runtime-v2","version":10}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP INDEX IF EXISTS interview_turn_decisions_user_idx;
DROP INDEX IF EXISTS interview_turn_decisions_session_idx;
DROP INDEX IF EXISTS interview_turn_decisions_pending_idx;
DROP TABLE IF EXISTS interview_turn_decisions;

ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_material_kind_chk;
ALTER TABLE questions DROP COLUMN IF EXISTS material_kind;

ALTER TABLE interview_turns DROP CONSTRAINT IF EXISTS interview_turns_invocation_id_fkey;
ALTER TABLE interview_turns DROP CONSTRAINT IF EXISTS interview_turns_generation_mode_chk;
ALTER TABLE interview_turns DROP CONSTRAINT IF EXISTS interview_turns_difficulty_v2_chk;
ALTER TABLE interview_turns
    DROP COLUMN IF EXISTS generation_mode,
    DROP COLUMN IF EXISTS coverage_observation,
    DROP COLUMN IF EXISTS decision_reason,
    DROP COLUMN IF EXISTS evidence_fact_ids,
    DROP COLUMN IF EXISTS difficulty,
    DROP COLUMN IF EXISTS expected_points,
    DROP COLUMN IF EXISTS intent;

DROP INDEX IF EXISTS interview_sessions_user_phase_updated_idx;
ALTER TABLE interview_sessions DROP CONSTRAINT IF EXISTS interview_sessions_status_phase_chk;
ALTER TABLE interview_sessions DROP CONSTRAINT IF EXISTS interview_sessions_policy_bounds_chk;
ALTER TABLE interview_sessions DROP CONSTRAINT IF EXISTS interview_sessions_phase_chk;
ALTER TABLE interview_sessions DROP CONSTRAINT IF EXISTS interview_sessions_agent_mode_chk;
ALTER TABLE interview_sessions
    DROP COLUMN IF EXISTS decision_version,
    DROP COLUMN IF EXISTS completion_reason,
    DROP COLUMN IF EXISTS capability_progress,
    DROP COLUMN IF EXISTS max_follow_ups_total,
    DROP COLUMN IF EXISTS max_follow_up_depth,
    DROP COLUMN IF EXISTS time_budget_minutes,
    DROP COLUMN IF EXISTS max_turns,
    DROP COLUMN IF EXISTS target_turns,
    DROP COLUMN IF EXISTS min_turns,
    DROP COLUMN IF EXISTS interviewer_prompt_version,
    DROP COLUMN IF EXISTS policy_version,
    DROP COLUMN IF EXISTS phase,
    DROP COLUMN IF EXISTS agent_mode;

UPDATE app_schema_metadata
SET value = '{"name":"launch-redesign","version":9}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';
