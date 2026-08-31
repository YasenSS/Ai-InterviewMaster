-- +goose Up
CREATE TABLE invite_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash text NOT NULL UNIQUE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    bound_email text,
    max_uses integer NOT NULL DEFAULT 1,
    used_count integer NOT NULL DEFAULT 0,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    CONSTRAINT invite_codes_max_uses_chk CHECK (max_uses BETWEEN 1 AND 10000),
    CONSTRAINT invite_codes_used_count_chk CHECK (used_count BETWEEN 0 AND max_uses),
    CONSTRAINT invite_codes_bound_email_chk CHECK (bound_email IS NULL OR length(trim(bound_email)) > 0)
);

CREATE INDEX invite_codes_expires_idx
    ON invite_codes (expires_at)
    WHERE expires_at IS NOT NULL;

UPDATE app_schema_metadata
SET value = '{"name":"invite-codes","version":11}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';

-- +goose Down
DROP INDEX IF EXISTS invite_codes_expires_idx;
DROP TABLE IF EXISTS invite_codes;

UPDATE app_schema_metadata
SET value = '{"name":"agent-runtime-v2","version":10}'::jsonb,
    updated_at = now()
WHERE key = 'milestone';
