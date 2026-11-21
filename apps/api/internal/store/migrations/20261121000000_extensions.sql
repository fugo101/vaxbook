-- Hand-written: Atlas cannot express Postgres extensions from the ent schema.
-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose Down
DROP EXTENSION IF EXISTS citext;
