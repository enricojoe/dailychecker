-- Migration 000004: occurrences table
-- Each occurrence is one "instance" of an activity on a specific Jakarta date.
-- state uses TEXT + CHECK (same rationale as activities.freq: simpler down migration).
-- The UNIQUE constraint on (activity_id, occur_date) enforces one occurrence per
-- activity per day and also serves as the index for that access pattern.
-- ON DELETE CASCADE: occurrences are historical records tied to the activity.

CREATE TABLE occurrences (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    activity_id              UUID        NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    occur_date               DATE        NOT NULL,
    state                    TEXT        NOT NULL DEFAULT 'pending'
                                         CHECK (state IN ('pending', 'partial', 'done')),
    completed_at             TIMESTAMPTZ,
    -- Notification sentinels used by the scheduler (Milestone 6)
    per_activity_notified_at TIMESTAMPTZ,
    digest_notified_at       TIMESTAMPTZ,
    CONSTRAINT occurrences_activity_date_unique UNIQUE (activity_id, occur_date)
);

-- Digest query: "find all not-done occurrences for a given date" (scans by date+state)
CREATE INDEX idx_occurrences_occur_date_state ON occurrences(occur_date, state);
