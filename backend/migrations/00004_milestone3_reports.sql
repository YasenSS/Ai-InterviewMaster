-- +goose Up
CREATE TABLE interview_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL UNIQUE REFERENCES interview_sessions(id) ON DELETE CASCADE,
    overall_score smallint NOT NULL CHECK (overall_score BETWEEN 0 AND 100),
    strengths jsonb NOT NULL DEFAULT '[]'::jsonb,
    improvements jsonb NOT NULL DEFAULT '[]'::jsonb,
    next_steps jsonb NOT NULL DEFAULT '[]'::jsonb,
    quality_gate jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE interview_turn_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES interview_reports(id) ON DELETE CASCADE,
    turn_id uuid NOT NULL UNIQUE REFERENCES interview_turns(id) ON DELETE CASCADE,
    score smallint NOT NULL CHECK (score BETWEEN 0 AND 100),
    critique text NOT NULL,
    golden_answer text NOT NULL,
    evidence jsonb NOT NULL DEFAULT '[]'::jsonb
);

-- +goose Down
DROP TABLE IF EXISTS interview_turn_reports;
DROP TABLE IF EXISTS interview_reports;
