-- Migration 000005: replace phone with username as the login identifier.
--
-- Safe on a populated table: username is added nullable first, backfilled,
-- then promoted to NOT NULL + UNIQUE, then phone is dropped.

-- Step 1: add username as nullable so existing rows are not immediately rejected.
ALTER TABLE users ADD COLUMN username TEXT;

-- Step 2: backfill existing rows with a deterministic value.
UPDATE users SET username = 'user_' || id::text WHERE username IS NULL;

-- Step 3: enforce NOT NULL now that every row has a value.
ALTER TABLE users ALTER COLUMN username SET NOT NULL;

-- Step 4: add unique index (named explicitly so the down migration can drop it by name).
CREATE UNIQUE INDEX users_username_unique ON users (username);

-- Step 5: drop the phone unique constraint and column.
-- The constraint created by "phone TEXT NOT NULL UNIQUE" in migration 000001
-- is named "users_phone_key" by Postgres (the default name for a UNIQUE
-- constraint is <table>_<column>_key).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;
ALTER TABLE users DROP COLUMN IF EXISTS phone;
