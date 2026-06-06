-- Migration 007: Add simplified review columns for new FE contract
-- Renames energy_level INT to energy_level_int, adds new string-based energy_level
-- Adds satisfaction, reflection, tomorrow_priority, ai_summary columns
-- Relaxes old column constraints for backward compatibility

-- Rename old energy_level INT to energy_level_int (preserves historical data)
ALTER TABLE daily_reviews RENAME COLUMN energy_level TO energy_level_int;

-- Add new simplified columns
ALTER TABLE daily_reviews ADD COLUMN energy_level VARCHAR(20);
ALTER TABLE daily_reviews ADD COLUMN satisfaction INT DEFAULT 0;
ALTER TABLE daily_reviews ADD COLUMN reflection TEXT;
ALTER TABLE daily_reviews ADD COLUMN tomorrow_priority VARCHAR(20);
ALTER TABLE daily_reviews ADD COLUMN ai_summary TEXT;

-- Relax old column constraints (make nullable for backward compatibility)
ALTER TABLE daily_reviews ALTER COLUMN difficulty_rating DROP NOT NULL;
ALTER TABLE daily_reviews ALTER COLUMN satisfaction_level DROP NOT NULL;
