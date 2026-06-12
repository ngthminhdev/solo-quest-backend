ALTER TABLE daily_quest_generation_jobs
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS existing_count,
    DROP COLUMN IF EXISTS target_count,
    DROP COLUMN IF EXISTS error_code;
