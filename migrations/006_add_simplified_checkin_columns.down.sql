-- Rollback: remove simplified daily check-in columns
-- Note: This is lossy - data in mood, availability, priority will be lost.

ALTER TABLE daily_checkins DROP COLUMN IF EXISTS mood;
ALTER TABLE daily_checkins DROP COLUMN IF EXISTS availability;
ALTER TABLE daily_checkins DROP COLUMN IF EXISTS priority;
