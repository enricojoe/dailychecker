-- Migration 000001: users table
-- gen_random_uuid() is built-in since PostgreSQL 13; no extension required.

CREATE TABLE users (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 TEXT        NOT NULL,
    -- phone is the primary login identifier; must be globally unique
    phone                TEXT        NOT NULL UNIQUE,
    password_hash        TEXT        NOT NULL,
    -- Telegram fields populated via the /start deep-link flow (Milestone 5)
    telegram_chat_id     BIGINT,
    telegram_link_token  TEXT,
    telegram_linked_at   TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
