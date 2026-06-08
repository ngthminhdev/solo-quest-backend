-- Reverse of 001_init_schema (consolidated)
-- Drops all 21 tables in FK-safe order.
-- WARNING: All data will be permanently lost.

DROP TABLE IF EXISTS user_learning_roadmap_step_progress CASCADE;
DROP TABLE IF EXISTS user_learning_roadmaps CASCADE;
DROP TABLE IF EXISTS learning_roadmap_steps CASCADE;
DROP TABLE IF EXISTS learning_roadmaps CASCADE;
DROP TABLE IF EXISTS quest_actions CASCADE;
DROP TABLE IF EXISTS xp_transactions CASCADE;
DROP TABLE IF EXISTS log_entries CASCADE;
DROP TABLE IF EXISTS reward_redemptions CASCADE;
DROP TABLE IF EXISTS rewards CASCADE;
DROP TABLE IF EXISTS daily_checkins CASCADE;
DROP TABLE IF EXISTS daily_reviews CASCADE;
DROP TABLE IF EXISTS onboarding_answers CASCADE;
DROP TABLE IF EXISTS quests CASCADE;
DROP TABLE IF EXISTS reminder_settings CASCADE;
DROP TABLE IF EXISTS quest_settings CASCADE;
DROP TABLE IF EXISTS schedule_blocks CASCADE;
DROP TABLE IF EXISTS app_settings CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS auth_accounts CASCADE;
DROP TABLE IF EXISTS user_profiles CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;
