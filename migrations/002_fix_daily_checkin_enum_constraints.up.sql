-- Fix daily_checkins enum fields that may have old numeric check constraints
-- from a previous GORM AutoMigrate that set these columns as numeric/rating style.
-- This migration preserves all existing data.

-- 1. Drop incompatible check constraints on daily_checkins if they exist
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT con.conname
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conparentid
        WHERE rel.relname = 'daily_checkins'
          AND con.contype = 'c'
          AND pg_get_constraintdef(con.oid) ~ '(energy_level|stress_level|focus_level|day_intensity)'
    LOOP
        EXECUTE 'ALTER TABLE daily_checkins DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
        RAISE NOTICE 'Dropped constraint % from daily_checkins', r.conname;
    END LOOP;
END $$;

-- 2. Drop incompatible check constraints on daily_reviews if they exist
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT con.conname
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conparentid
        WHERE rel.relname = 'daily_reviews'
          AND con.contype = 'c'
          AND pg_get_constraintdef(con.oid) ~ '(mood|difficulty_rating|energy_level|satisfaction_level)'
    LOOP
        EXECUTE 'ALTER TABLE daily_reviews DROP CONSTRAINT IF EXISTS ' || quote_ident(r.conname);
        RAISE NOTICE 'Dropped constraint % from daily_reviews', r.conname;
    END LOOP;
END $$;

-- 3. Convert numeric-like values in daily_checkins to enum strings
-- energy_level, stress_level, focus_level: 1->veryLow, 2->low, 3->medium, 4->high, 5->veryHigh
-- day_intensity: 1->light, 2->normal, 3->busy, 4->overloaded

-- Only convert if the column currently holds numeric-looking values (digits only)
DO $$
BEGIN
    -- energy_level
    IF EXISTS (
        SELECT 1 FROM daily_checkins
        WHERE energy_level ~ '^[0-9]+$'
        LIMIT 1
    ) THEN
        ALTER TABLE daily_checkins ALTER COLUMN energy_level TYPE VARCHAR(20);
        UPDATE daily_checkins SET energy_level = CASE energy_level
            WHEN '1' THEN 'veryLow'
            WHEN '2' THEN 'low'
            WHEN '3' THEN 'medium'
            WHEN '4' THEN 'high'
            WHEN '5' THEN 'veryHigh'
            ELSE energy_level
        END
        WHERE energy_level ~ '^[0-9]+$';
        RAISE NOTICE 'Converted daily_checkins.energy_level from numeric to enum strings';
    END IF;

    -- stress_level
    IF EXISTS (
        SELECT 1 FROM daily_checkins
        WHERE stress_level ~ '^[0-9]+$'
        LIMIT 1
    ) THEN
        ALTER TABLE daily_checkins ALTER COLUMN stress_level TYPE VARCHAR(20);
        UPDATE daily_checkins SET stress_level = CASE stress_level
            WHEN '1' THEN 'veryLow'
            WHEN '2' THEN 'low'
            WHEN '3' THEN 'medium'
            WHEN '4' THEN 'high'
            WHEN '5' THEN 'veryHigh'
            ELSE stress_level
        END
        WHERE stress_level ~ '^[0-9]+$';
        RAISE NOTICE 'Converted daily_checkins.stress_level from numeric to enum strings';
    END IF;

    -- focus_level
    IF EXISTS (
        SELECT 1 FROM daily_checkins
        WHERE focus_level ~ '^[0-9]+$'
        LIMIT 1
    ) THEN
        ALTER TABLE daily_checkins ALTER COLUMN focus_level TYPE VARCHAR(20);
        UPDATE daily_checkins SET focus_level = CASE focus_level
            WHEN '1' THEN 'veryLow'
            WHEN '2' THEN 'low'
            WHEN '3' THEN 'medium'
            WHEN '4' THEN 'high'
            WHEN '5' THEN 'veryHigh'
            ELSE focus_level
        END
        WHERE focus_level ~ '^[0-9]+$';
        RAISE NOTICE 'Converted daily_checkins.focus_level from numeric to enum strings';
    END IF;

    -- day_intensity
    IF EXISTS (
        SELECT 1 FROM daily_checkins
        WHERE day_intensity ~ '^[0-9]+$'
        LIMIT 1
    ) THEN
        ALTER TABLE daily_checkins ALTER COLUMN day_intensity TYPE VARCHAR(20);
        UPDATE daily_checkins SET day_intensity = CASE day_intensity
            WHEN '1' THEN 'light'
            WHEN '2' THEN 'normal'
            WHEN '3' THEN 'busy'
            WHEN '4' THEN 'overloaded'
            ELSE day_intensity
        END
        WHERE day_intensity ~ '^[0-9]+$';
        RAISE NOTICE 'Converted daily_checkins.day_intensity from numeric to enum strings';
    END IF;
END $$;

-- 4. Ensure columns are varchar(20) even if no numeric data existed
-- (handles case where column type was changed by GORM but no data to convert)
DO $$
BEGIN
    -- daily_checkins columns
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN energy_level TYPE VARCHAR(20); EXCEPTION WHEN OTHERS THEN NULL; END;
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN stress_level TYPE VARCHAR(20); EXCEPTION WHEN OTHERS THEN NULL; END;
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN focus_level TYPE VARCHAR(20); EXCEPTION WHEN OTHERS THEN NULL; END;
    BEGIN ALTER TABLE daily_checkins ALTER COLUMN day_intensity TYPE VARCHAR(20); EXCEPTION WHEN OTHERS THEN NULL; END;

    -- daily_reviews.mood should be varchar(20)
    BEGIN ALTER TABLE daily_reviews ALTER COLUMN mood TYPE VARCHAR(20); EXCEPTION WHEN OTHERS THEN NULL; END;
END $$;
