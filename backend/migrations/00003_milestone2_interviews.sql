-- +goose Up
CREATE TYPE question_set_status AS ENUM ('ready', 'archived');
CREATE TYPE interview_status AS ENUM ('draft', 'active', 'completed', 'abandoned');

CREATE TABLE question_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resume_id uuid NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    job_description_id uuid REFERENCES job_descriptions(id) ON DELETE SET NULL,
    target_role text,
    status question_set_status NOT NULL DEFAULT 'ready',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX question_sets_user_created_idx ON question_sets (user_id, created_at DESC);

CREATE TABLE questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_set_id uuid NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
    ordinal integer NOT NULL,
    question text NOT NULL,
    intent text NOT NULL,
    expected_points jsonb NOT NULL DEFAULT '[]'::jsonb,
    evidence_fact_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    follow_up_hint text,
    UNIQUE (question_set_id, ordinal)
);

CREATE TABLE interview_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resume_id uuid NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    question_set_id uuid REFERENCES question_sets(id) ON DELETE SET NULL,
    job_description_id uuid REFERENCES job_descriptions(id) ON DELETE SET NULL,
    title text NOT NULL,
    status interview_status NOT NULL DEFAULT 'draft',
    current_ordinal integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX interview_sessions_user_updated_idx ON interview_sessions (user_id, updated_at DESC);

CREATE TABLE interview_turns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    ordinal integer NOT NULL,
    question text NOT NULL,
    answer text,
    created_at timestamptz NOT NULL DEFAULT now(),
    answered_at timestamptz,
    UNIQUE (session_id, ordinal)
);

-- +goose Down
DROP TABLE IF EXISTS interview_turns;
DROP TABLE IF EXISTS interview_sessions;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS question_sets;
DROP TYPE IF EXISTS interview_status;
DROP TYPE IF EXISTS question_set_status;
