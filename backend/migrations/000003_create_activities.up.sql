-- Migration 000003: activities table
-- Activities are schedule templates. Sub-activities share the parent's schedule.
-- freq uses a TEXT CHECK constraint rather than an enum type so that down migrations
-- are simple DROP TABLE (no enum type cleanup required).
-- days_of_week is an integer array where 0=Sunday … 6=Saturday, matching the
-- robfig/cron convention. Empty array '{}' for daily activities (NOT NULL).
-- ON DELETE CASCADE on user_id and parent_id: deleting a user removes their
-- activities; deleting a parent removes its children.

CREATE TABLE activities (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id    UUID        REFERENCES activities(id) ON DELETE CASCADE,
    title        TEXT        NOT NULL,
    notes        TEXT,
    freq         TEXT        NOT NULL CHECK (freq IN ('daily', 'weekly')),
    -- Empty array {} for daily activities; non-empty for weekly.
    days_of_week INTEGER[]   NOT NULL DEFAULT '{}',
    -- Stored as local Jakarta time-of-day (v1 assumption: single global timezone).
    time_of_day  TIME        NOT NULL,
    sort_order   INTEGER     NOT NULL DEFAULT 0,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary query path for "list active activities for a user"
CREATE INDEX idx_activities_user_id_is_active ON activities(user_id, is_active);
-- Needed for parent→children lookups and cascade correctness
CREATE INDEX idx_activities_parent_id ON activities(parent_id);
