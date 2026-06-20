-- Migration 000002: refresh_tokens table
-- Tokens are stored as bcrypt hashes (token_hash); the raw token is never persisted.
-- ON DELETE CASCADE: tokens are meaningless without their owning user.

CREATE TABLE refresh_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    -- revoked_at is NULL while the token is valid; set when explicitly revoked.
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Speed up lookups by user (e.g. "list all active tokens for user X")
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
