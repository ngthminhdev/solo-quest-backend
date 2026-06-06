DROP TABLE IF EXISTS reminder_settings;

ALTER TABLE app_settings DROP COLUMN IF EXISTS quiet_hours_enabled;
ALTER TABLE app_settings DROP COLUMN IF EXISTS quiet_start_time;
ALTER TABLE app_settings DROP COLUMN IF EXISTS quiet_end_time;
