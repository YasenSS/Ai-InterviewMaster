-- name: GetDatabaseTime :one
SELECT now()::timestamptz;

-- name: GetSchemaMetadata :one
SELECT value
FROM app_schema_metadata
WHERE key = $1;
