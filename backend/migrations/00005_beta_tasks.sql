-- +goose Up
ALTER TABLE async_tasks ADD COLUMN result jsonb;

-- +goose Down
ALTER TABLE async_tasks DROP COLUMN IF EXISTS result;
