-- Rollback migration 007: Remove simplified review columns

-- Drop new columns
ALTER TABLE daily_reviews DROP COLUMN IF EXISTS ai_summary;
ALTER TABLE daily_reviews DROP COLUMN IF EXISTS tomorrow_priority;
ALTER TABLE daily_reviews DROP COLUMN IF EXISTS reflection;
ALTER TABLE daily_reviews DROP COLUMN IF EXISTS satisfaction;
ALTER TABLE daily_reviews DROP COLUMN IF EXISTS energy_level;

-- Rename energy_level_int back to energy_level
ALTER TABLE daily_reviews RENAME COLUMN energy_level_int TO energy_level;
