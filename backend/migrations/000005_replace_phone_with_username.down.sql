-- Migration 000005 (down): restore phone, remove username.
--
-- Mirrors the up migration in reverse. The restored phone column is nullable
-- so the down can run on a populated table (there is no meaningful phone data
-- to backfill; callers must treat it as unknown after a down rollback).

-- Step 1: re-add phone as nullable (mirroring original 000001 column type).
ALTER TABLE users ADD COLUMN phone TEXT;

-- Step 2: backfill a deterministic placeholder so NOT NULL can be enforced.
UPDATE users SET phone = 'unknown_' || id::text WHERE phone IS NULL;

-- Step 3: promote to NOT NULL.
ALTER TABLE users ALTER COLUMN phone SET NOT NULL;

-- Step 4: restore unique constraint with the same name Postgres used in 000001.
ALTER TABLE users ADD CONSTRAINT users_phone_key UNIQUE (phone);

-- Step 5: drop the username unique index and column.
DROP INDEX IF EXISTS users_username_unique;
ALTER TABLE users DROP COLUMN IF EXISTS username;
