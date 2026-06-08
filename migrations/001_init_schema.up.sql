-- SoloQuest consolidated baseline schema
-- Source of truth: SQL migrations (not GORM AutoMigrate)
-- Consolidates original migrations 001 through 014 into a single clean file.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ──────────────────────────────────────────────────────────────────────────────
-- 1. user_profiles (41 columns — base + onboarding/scheduling fields)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity / profile
    display_name VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(500),
    age INT,
    gender VARCHAR(20),
    height_cm DOUBLE PRECISION,
    weight_kg DOUBLE PRECISION,
    main_activity VARCHAR(100),

    -- Gamification / progress
    level INT DEFAULT 1,
    current_level_exp INT DEFAULT 0,
    next_level_exp INT DEFAULT 100,
    total_exp INT DEFAULT 0,
    reward_points INT DEFAULT 0,
    streak_days INT DEFAULT 0,
    best_streak INT DEFAULT 0,
    streak_shields INT DEFAULT 0,
    total_completed_quests INT DEFAULT 0,
    total_skipped_quests INT DEFAULT 0,

    -- Onboarding
    has_completed_onboarding BOOLEAN DEFAULT FALSE,
    quiet_after_time VARCHAR(10),
    main_goals JSONB,
    health_limitations JSONB,
    preferred_rewards JSONB,

    -- Work schedule (from 014)
    work_schedule_type VARCHAR(50),
    work_weekdays JSONB,
    work_start_time VARCHAR(10),
    work_end_time VARCHAR(10),

    -- Free time preferences (from 014)
    preferred_free_times JSONB,
    free_time_preference VARCHAR(50),

    -- Learning time preferences (from 014)
    learning_time_preferences JSONB,
    learning_time_preference VARCHAR(50),

    -- Movement time preferences (from 014)
    movement_time_preferences JSONB,
    movement_time_preference VARCHAR(50),

    -- Other time preferences (from 014)
    sleep_time_preference VARCHAR(50),
    nutrition_time_preference VARCHAR(50),

    -- Health / activity (from 014)
    activity_level VARCHAR(50),
    last_workout VARCHAR(50),

    -- Daily rhythm (from 014)
    wake_up_time VARCHAR(10),
    target_sleep_time VARCHAR(10),

    -- Learning (from 014)
    learning_topic VARCHAR(255),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ──────────────────────────────────────────────────────────────────────────────
-- 2. auth_accounts
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS auth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    provider VARCHAR(20) NOT NULL,
    provider_uid VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_auth_accounts_provider_uid UNIQUE (provider, provider_uid)
);
CREATE INDEX IF NOT EXISTS idx_auth_accounts_user_id ON auth_accounts(user_id);

-- ──────────────────────────────────────────────────────────────────────────────
-- 3. user_sessions
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active ON user_sessions(user_id, expires_at) WHERE revoked_at IS NULL;

-- ──────────────────────────────────────────────────────────────────────────────
-- 4. onboarding_answers
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS onboarding_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    answers JSONB NOT NULL,
    completed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_onboarding_answers_user_id UNIQUE (user_id)
);

-- ──────────────────────────────────────────────────────────────────────────────
-- 5. app_settings
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS app_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    locale VARCHAR(10) DEFAULT 'en',
    theme VARCHAR(20) DEFAULT 'system',
    daily_quest_limit INT DEFAULT 10,
    notifications_enabled BOOLEAN DEFAULT TRUE,
    quiet_after_time VARCHAR(10) DEFAULT '22:00',
    quiet_hours_enabled BOOLEAN DEFAULT FALSE,
    quiet_start_time VARCHAR(10),
    quiet_end_time VARCHAR(10),
    daily_reminder_time VARCHAR(10),
    weekly_review_day VARCHAR(10),
    timezone VARCHAR(50) DEFAULT 'UTC',
    preferences JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_app_settings_user_id UNIQUE (user_id)
);

-- ──────────────────────────────────────────────────────────────────────────────
-- 6. quest_settings
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS quest_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    daily_quest_count INT NOT NULL DEFAULT 8,
    difficulty VARCHAR(20) NOT NULL DEFAULT 'normal',
    auto_adjust_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    enabled_categories JSONB NOT NULL DEFAULT '["water","breakTime","movement","learning","sleep","review"]',
    preferred_duration VARCHAR(20) NOT NULL DEFAULT 'medium',
    rest_day_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rules JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_quest_settings_user_id UNIQUE (user_id)
);
CREATE INDEX IF NOT EXISTS idx_quest_settings_user_id ON quest_settings(user_id);

-- ──────────────────────────────────────────────────────────────────────────────
-- 7. schedule_blocks
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS schedule_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    title VARCHAR(200) NOT NULL,
    type VARCHAR(20) NOT NULL,
    days_of_week JSONB NOT NULL,
    start_time VARCHAR(5) NOT NULL,
    end_time VARCHAR(5) NOT NULL,
    is_busy BOOLEAN NOT NULL DEFAULT FALSE,
    is_flexible BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    location VARCHAR(300),
    note TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_schedule_blocks_user_id ON schedule_blocks(user_id);
CREATE INDEX IF NOT EXISTS idx_schedule_blocks_type ON schedule_blocks(type);
CREATE INDEX IF NOT EXISTS idx_schedule_blocks_enabled ON schedule_blocks(enabled);
CREATE INDEX IF NOT EXISTS idx_schedule_blocks_user_enabled ON schedule_blocks(user_id, enabled);

-- ──────────────────────────────────────────────────────────────────────────────
-- 8. learning_roadmaps
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS learning_roadmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(50) NOT NULL,
    difficulty VARCHAR(20) NOT NULL DEFAULT 'normal',
    estimated_minutes INT NOT NULL DEFAULT 0,
    total_steps INT NOT NULL DEFAULT 0,
    source VARCHAR(20) NOT NULL DEFAULT 'system',
    created_by_user_id UUID REFERENCES user_profiles(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_enabled ON learning_roadmaps(enabled);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_category ON learning_roadmaps(category);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_source ON learning_roadmaps(source);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_created_by_user_id ON learning_roadmaps(created_by_user_id);

-- ──────────────────────────────────────────────────────────────────────────────
-- 9. learning_roadmap_steps
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS learning_roadmap_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id UUID NOT NULL REFERENCES learning_roadmaps(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    order_index INT NOT NULL,
    estimated_minutes INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_steps_roadmap_id ON learning_roadmap_steps(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_steps_order ON learning_roadmap_steps(roadmap_id, order_index);

-- ──────────────────────────────────────────────────────────────────────────────
-- 10. user_learning_roadmaps
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_learning_roadmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    roadmap_id UUID NOT NULL REFERENCES learning_roadmaps(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'tracking',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, roadmap_id)
);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_user_id ON user_learning_roadmaps(user_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_roadmap_id ON user_learning_roadmaps(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_status ON user_learning_roadmaps(user_id, status);

-- ──────────────────────────────────────────────────────────────────────────────
-- 11. user_learning_roadmap_step_progress
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_learning_roadmap_step_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    roadmap_id UUID NOT NULL REFERENCES learning_roadmaps(id) ON DELETE CASCADE,
    step_id UUID NOT NULL REFERENCES learning_roadmap_steps(id) ON DELETE CASCADE,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, step_id)
);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_user_id ON user_learning_roadmap_step_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_roadmap_id ON user_learning_roadmap_step_progress(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_step_id ON user_learning_roadmap_step_progress(step_id);

-- ──────────────────────────────────────────────────────────────────────────────
-- 12. quests
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS quests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    difficulty VARCHAR(20) DEFAULT 'easy',
    source VARCHAR(20) DEFAULT 'dailyPlan',
    xp_reward INT DEFAULT 0,
    estimated_minutes INT DEFAULT 0,
    reason TEXT,
    instruction TEXT,
    tags JSONB,
    available_time_blocks JSONB,
    date DATE NOT NULL,
    due_date DATE,
    reminder_time TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    snoozed_until TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quests_user_date ON quests(user_id, date);
CREATE INDEX IF NOT EXISTS idx_quests_user_status ON quests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_quests_user_type ON quests(user_id, type);
CREATE INDEX IF NOT EXISTS idx_quests_type ON quests(type);

-- ──────────────────────────────────────────────────────────────────────────────
-- 13. quest_actions
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS quest_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quest_id UUID NOT NULL REFERENCES quests(id),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    action VARCHAR(20) NOT NULL,
    note TEXT,
    reason TEXT,
    snooze_minutes INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quest_actions_quest_id ON quest_actions(quest_id);
CREATE INDEX IF NOT EXISTS idx_quest_actions_user_created ON quest_actions(user_id, created_at);

-- ──────────────────────────────────────────────────────────────────────────────
-- 14. daily_checkins
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS daily_checkins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    date DATE NOT NULL,
    mood VARCHAR(20),
    energy_level VARCHAR(20),
    stress_level VARCHAR(20),
    focus_level VARCHAR(20),
    day_intensity VARCHAR(20),
    availability VARCHAR(20),
    priority VARCHAR(20),
    main_focus_today TEXT,
    note TEXT,
    available_time_blocks JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_daily_checkins_user_date UNIQUE (user_id, date)
);

-- ──────────────────────────────────────────────────────────────────────────────
-- 15. daily_reviews
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS daily_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    date DATE NOT NULL,
    mood VARCHAR(20),
    energy_level VARCHAR(20),
    energy_level_int INT DEFAULT 0,
    satisfaction INT DEFAULT 0,
    reflection TEXT,
    tomorrow_priority VARCHAR(20),
    ai_summary TEXT,
    difficulty_rating INT DEFAULT 0,
    satisfaction_level INT DEFAULT 0,
    completed_quest_count INT DEFAULT 0,
    skipped_quest_count INT DEFAULT 0,
    earned_exp INT DEFAULT 0,
    completion_rate DOUBLE PRECISION,
    helpful_quests JSONB,
    annoying_quests JSONB,
    best_moment TEXT,
    challenge TEXT,
    improvement_tomorrow TEXT,
    tomorrow_adjustments JSONB,
    note TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_daily_reviews_user_date UNIQUE (user_id, date)
);

-- ──────────────────────────────────────────────────────────────────────────────
-- 16. log_entries
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS log_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    metadata JSONB,
    quest_id UUID,
    quest_type VARCHAR(20),
    exp_changed INT DEFAULT 0,
    points_changed INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_log_entries_user_created ON log_entries(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_log_entries_user_type ON log_entries(user_id, type);
CREATE INDEX IF NOT EXISTS idx_log_entries_type ON log_entries(type);

-- ──────────────────────────────────────────────────────────────────────────────
-- 17. xp_transactions
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS xp_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    amount INT NOT NULL,
    currency VARCHAR(20) NOT NULL,
    source VARCHAR(20) NOT NULL,
    source_id UUID,
    reference_id UUID,
    description TEXT,
    balance_after INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_xp_transactions_user_created ON xp_transactions(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_xp_transactions_user_currency ON xp_transactions(user_id, currency);
CREATE INDEX IF NOT EXISTS idx_xp_transactions_currency ON xp_transactions(currency);

-- ──────────────────────────────────────────────────────────────────────────────
-- 18. rewards
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS rewards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(20) NOT NULL,
    cost_points INT NOT NULL,
    icon_text VARCHAR(10),
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    image_url VARCHAR(500),
    duration_minutes INT,
    cooldown_minutes INT,
    claim_count INT,
    claimed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_rewards_user_status ON rewards(user_id, status);

-- ──────────────────────────────────────────────────────────────────────────────
-- 19. reward_redemptions
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS reward_redemptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    reward_id UUID NOT NULL REFERENCES rewards(id),
    cost INT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reward_redemptions_user_created ON reward_redemptions(user_id, created_at);

-- ──────────────────────────────────────────────────────────────────────────────
-- 20. reminder_settings
-- ──────────────────────────────────────────────────────────────────────────────
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

-- ──────────────────────────────────────────────────────────────────────────────
-- 21. schema_migrations tracking (used by golang-migrate)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE
);
