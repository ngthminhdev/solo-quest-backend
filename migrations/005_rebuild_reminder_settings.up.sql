-- Rebuild reminder_settings table to match FE ReminderSettingModel contract.
-- Drops the old table (from migration 004) and creates a new one with
-- FE-aligned columns: type, title, description, frequency, status,
-- start_time, end_time, interval_minutes, max_per_day, smart_enabled.

DROP TABLE IF EXISTS reminder_settings CASCADE;

CREATE TABLE IF NOT EXISTS reminder_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    type VARCHAR(30) NOT NULL,
    title VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    frequency VARCHAR(20) NOT NULL DEFAULT 'fixed',
    status VARCHAR(10) NOT NULL DEFAULT 'enabled',
    start_time VARCHAR(5),
    end_time VARCHAR(5),
    interval_minutes INT,
    max_per_day INT,
    smart_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_reminder_settings_user_type UNIQUE (user_id, type)
);

CREATE INDEX IF NOT EXISTS idx_reminder_settings_user_id ON reminder_settings(user_id);
CREATE INDEX IF NOT EXISTS idx_reminder_settings_type ON reminder_settings(type);
