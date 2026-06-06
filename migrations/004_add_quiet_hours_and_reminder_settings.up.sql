-- Add quiet hours fields to app_settings
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS quiet_hours_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS quiet_start_time VARCHAR(10);
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS quiet_end_time VARCHAR(10);

-- Create reminder_settings table
CREATE TABLE IF NOT EXISTS reminder_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    quest_type VARCHAR(20) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    frequency VARCHAR(20) NOT NULL DEFAULT 'daily',
    interval_minutes INT,
    time_range_start VARCHAR(10),
    time_range_end VARCHAR(10),
    reminder_time VARCHAR(10),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_reminder_settings_user_quest_type UNIQUE (user_id, quest_type)
);

CREATE INDEX IF NOT EXISTS idx_reminder_settings_user_id ON reminder_settings(user_id);
