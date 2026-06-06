-- Add new simplified daily check-in columns: mood, availability, priority
-- These support the new simplified daily check-in contract.

-- 1. Add mood column
ALTER TABLE daily_checkins ADD COLUMN IF NOT EXISTS mood VARCHAR(20);

-- 2. Add availability column
ALTER TABLE daily_checkins ADD COLUMN IF NOT EXISTS availability VARCHAR(20);

-- 3. Add priority column
ALTER TABLE daily_checkins ADD COLUMN IF NOT EXISTS priority VARCHAR(20);

-- 4. Make old columns nullable (they are no longer required for new check-ins)
-- These columns already exist but may have been created as NOT NULL by GORM.
-- We relax them so new simplified check-ins don't need them.
DO $$
BEGIN
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN stress_level DROP NOT NULL; EXCEPTION WHEN OTHERS THEN NULL; END;
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN focus_level DROP NOT NULL; EXCEPTION WHEN OTHERS THEN NULL; END;
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN day_intensity DROP NOT NULL; EXCEPTION WHEN OTHERS THEN NULL; END;
END $$;
