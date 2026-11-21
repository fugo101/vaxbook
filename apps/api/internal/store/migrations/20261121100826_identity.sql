-- +goose Up
-- create "accounts" table
CREATE TABLE "accounts" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "email" citext NOT NULL, PRIMARY KEY ("id"));
-- create index "accounts_email_key" to table: "accounts"
CREATE UNIQUE INDEX "accounts_email_key" ON "accounts" ("email");
-- create "otp_challenges" table
CREATE TABLE "otp_challenges" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "email_hash" bytea NOT NULL, "code_hash" bytea NOT NULL, "expires_at" timestamptz NOT NULL, "resend_after" timestamptz NOT NULL, "attempts" bigint NOT NULL DEFAULT 0, PRIMARY KEY ("id"));
-- create index "otp_challenges_email_hash_key" to table: "otp_challenges"
CREATE UNIQUE INDEX "otp_challenges_email_hash_key" ON "otp_challenges" ("email_hash");
-- create "sessions" table
CREATE TABLE "sessions" ("id" character varying NOT NULL, "created_at" timestamptz NOT NULL, "last_seen_at" timestamptz NOT NULL, "expires_at" timestamptz NOT NULL, "account_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "sessions_accounts_sessions" FOREIGN KEY ("account_id") REFERENCES "accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "session_expires_at" to table: "sessions"
CREATE INDEX "session_expires_at" ON "sessions" ("expires_at");

-- +goose Down
-- reverse: create index "session_expires_at" to table: "sessions"
DROP INDEX "session_expires_at";
-- reverse: create "sessions" table
DROP TABLE "sessions";
-- reverse: create index "otp_challenges_email_hash_key" to table: "otp_challenges"
DROP INDEX "otp_challenges_email_hash_key";
-- reverse: create "otp_challenges" table
DROP TABLE "otp_challenges";
-- reverse: create index "accounts_email_key" to table: "accounts"
DROP INDEX "accounts_email_key";
-- reverse: create "accounts" table
DROP TABLE "accounts";
