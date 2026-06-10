-- 005: Add learning_metadata JSONB to quests
-- Stores roadmap_id / step_id linkage so learning quest completion
-- can drive Learning Roadmap progress.
ALTER TABLE quests ADD COLUMN IF NOT EXISTS learning_metadata JSONB;
