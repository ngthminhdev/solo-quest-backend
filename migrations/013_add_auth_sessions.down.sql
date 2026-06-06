DROP INDEX IF EXISTS idx_user_sessions_user_active;
DROP INDEX IF EXISTS idx_user_sessions_expires_at;
DROP INDEX IF EXISTS idx_user_sessions_user_id;

DROP TABLE IF EXISTS user_sessions;

ALTER TABLE auth_accounts
DROP COLUMN IF EXISTS last_login_at;
