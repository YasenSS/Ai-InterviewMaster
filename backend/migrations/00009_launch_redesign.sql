-- +goose Up
ALTER TYPE interview_status ADD VALUE IF NOT EXISTS 'preparing';
ALTER TYPE interview_status ADD VALUE IF NOT EXISTS 'failed';

ALTER TABLE interview_sessions
    ADD COLUMN IF NOT EXISTS primary_language text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_company text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS resume_version_id uuid REFERENCES resume_versions(id) ON DELETE RESTRICT;

ALTER TABLE question_sets
    ADD COLUMN IF NOT EXISTS primary_language text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_company text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS resume_version_id uuid REFERENCES resume_versions(id) ON DELETE RESTRICT;

UPDATE interview_sessions AS session
SET resume_version_id = resume.current_version_id
FROM resumes AS resume
WHERE session.resume_id = resume.id AND session.resume_version_id IS NULL;

UPDATE question_sets AS qset
SET resume_version_id = resume.current_version_id
FROM resumes AS resume
WHERE qset.resume_id = resume.id AND qset.resume_version_id IS NULL;

ALTER TABLE interview_sessions ALTER COLUMN resume_version_id SET NOT NULL;
ALTER TABLE question_sets ALTER COLUMN resume_version_id SET NOT NULL;

UPDATE question_sets AS qset
SET target_company = COALESCE(job.company, '')
FROM job_descriptions AS job
WHERE qset.job_description_id = job.id
  AND qset.target_company = '';

UPDATE interview_sessions AS session
SET target_company = COALESCE(job.company, '')
FROM job_descriptions AS job
WHERE session.job_description_id = job.id
  AND session.target_company = '';

UPDATE interview_sessions AS session
SET target_company = qset.target_company
FROM question_sets AS qset
WHERE session.question_set_id = qset.id
  AND session.target_company = ''
  AND qset.target_company <> '';

ALTER TABLE interview_sessions
    DROP CONSTRAINT IF EXISTS interview_sessions_job_description_id_fkey,
    DROP COLUMN IF EXISTS job_description_id;

ALTER TABLE question_sets
    DROP CONSTRAINT IF EXISTS question_sets_job_description_id_fkey,
    DROP COLUMN IF EXISTS job_description_id;

DROP TABLE IF EXISTS job_descriptions;

ALTER TABLE interview_turns
    ADD COLUMN IF NOT EXISTS source_question_id uuid REFERENCES questions(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS interview_turns_source_question_idx
    ON interview_turns (source_question_id)
    WHERE source_question_id IS NOT NULL;

-- Preserve the exact scoring rubric for turns that were already presented by
-- the legacy runner. Future, unanswered legacy turns were never shown to the
-- candidate and must not remain in the dynamic transcript.
UPDATE interview_turns AS turn
SET source_question_id = question.id
FROM interview_sessions AS session
JOIN questions AS question
  ON question.question_set_id = session.question_set_id
WHERE turn.session_id = session.id
  AND turn.ordinal = question.ordinal
  AND turn.source_question_id IS NULL;

DELETE FROM interview_turns AS turn
USING interview_sessions AS session
WHERE turn.session_id = session.id
  AND session.status = 'active'
  AND turn.ordinal > session.current_ordinal
  AND turn.answer IS NULL
  AND turn.skipped_at IS NULL;

UPDATE app_schema_metadata
SET value = '{"name":"launch-redesign","version":9}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP INDEX IF EXISTS interview_turns_source_question_idx;
ALTER TABLE interview_turns DROP COLUMN IF EXISTS source_question_id;

CREATE TABLE job_descriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company text,
    title text NOT NULL,
    content text NOT NULL,
    extracted_capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX job_descriptions_user_updated_idx ON job_descriptions (user_id, updated_at DESC);

ALTER TABLE question_sets
    DROP COLUMN IF EXISTS resume_version_id,
    DROP COLUMN IF EXISTS target_company,
    DROP COLUMN IF EXISTS primary_language,
    ADD COLUMN job_description_id uuid REFERENCES job_descriptions(id) ON DELETE SET NULL;

ALTER TABLE interview_sessions
    DROP COLUMN IF EXISTS resume_version_id,
    DROP COLUMN IF EXISTS target_company,
    DROP COLUMN IF EXISTS primary_language,
    ADD COLUMN job_description_id uuid REFERENCES job_descriptions(id) ON DELETE SET NULL;

UPDATE app_schema_metadata
SET value = '{"name":"agent-p1p2","version":8}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- PostgreSQL enum values are intentionally retained on rollback.
