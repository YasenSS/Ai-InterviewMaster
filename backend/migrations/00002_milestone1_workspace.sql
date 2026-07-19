-- +goose Up
CREATE TYPE resume_status AS ENUM ('draft', 'uploading', 'pending', 'processing', 'completed', 'failed');
CREATE TYPE task_status AS ENUM ('pending', 'running', 'succeeded', 'failed');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_normalized CHECK (email = lower(email))
);

CREATE TABLE resumes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL,
    status resume_status NOT NULL DEFAULT 'draft',
    current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX resumes_user_updated_idx ON resumes (user_id, updated_at DESC);

CREATE TABLE resume_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resume_id uuid NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    version_no integer NOT NULL,
    object_key text NOT NULL UNIQUE,
    original_filename text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    checksum_sha256 text,
    extracted_text text,
    parse_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    UNIQUE (resume_id, version_no)
);
ALTER TABLE resumes
    ADD CONSTRAINT resumes_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES resume_versions(id) ON DELETE SET NULL;

CREATE TABLE async_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_type text NOT NULL,
    ref_id uuid NOT NULL,
    status task_status NOT NULL DEFAULT 'pending',
    progress smallint NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX async_tasks_user_created_idx ON async_tasks (user_id, created_at DESC);
CREATE INDEX async_tasks_ref_idx ON async_tasks (ref_id, task_type);

CREATE TABLE resume_facts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resume_version_id uuid NOT NULL REFERENCES resume_versions(id) ON DELETE CASCADE,
    fact_type text NOT NULL,
    fact_key text NOT NULL,
    fact_value jsonb NOT NULL,
    source_excerpt text NOT NULL,
    confidence numeric(4,3) NOT NULL DEFAULT 1 CHECK (confidence BETWEEN 0 AND 1),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX resume_facts_version_type_idx ON resume_facts (resume_version_id, fact_type);

CREATE TABLE resume_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resume_version_id uuid NOT NULL REFERENCES resume_versions(id) ON DELETE CASCADE,
    chunk_no integer NOT NULL,
    content text NOT NULL,
    token_count integer NOT NULL DEFAULT 0,
    embedding vector(1536),
    embedding_model text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (resume_version_id, chunk_no)
);

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

UPDATE app_schema_metadata
SET value = '{"name":"milestone-1","version":2}'::jsonb, updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP TABLE IF EXISTS job_descriptions;
DROP TABLE IF EXISTS resume_chunks;
DROP TABLE IF EXISTS resume_facts;
DROP TABLE IF EXISTS async_tasks;
ALTER TABLE resumes DROP CONSTRAINT IF EXISTS resumes_current_version_fk;
DROP TABLE IF EXISTS resume_versions;
DROP TABLE IF EXISTS resumes;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS resume_status;
