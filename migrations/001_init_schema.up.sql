-- Baseline schema for SoloQuest backend
-- Source of truth: SQL migrations (not GORM AutoMigrate)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- user_profiles
CREATE TABLE IF NOT EXISTS user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(500),
    age INT,
    gender VARCHAR(20),
    height_cm DOUBLE PRECISION,
    weight_kg DOUBLE PRECISION,
    main_activity VARCHAR(100),
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
    has_completed_onboarding BOOLEAN DEFAULT FALSE,
    quiet_after_time VARCHAR(10),
    main_goals JSONB,
    health_limitations JSONB,
    preferred_rewards JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- auth_accounts
CREATE TABLE IF NOT EXISTS auth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    provider VARCHAR(20) NOT NULL,
    provider_uid VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_auth_accounts_provider_uid UNIQUE (provider, provider_uid)
);
CREATE INDEX IF NOT EXISTS idx_auth_accounts_user_id ON auth_accounts(user_id);

-- onboarding_answers
CREATE TABLE IF NOT EXISTS onboarding_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    answers JSONB NOT NULL,
    completed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_onboarding_answers_user_id UNIQUE (user_id)
);

-- quests
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

-- quest_actions
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

-- daily_checkins
CREATE TABLE IF NOT EXISTS daily_checkins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    date DATE NOT NULL,
    energy_level VARCHAR(20),
    stress_level VARCHAR(20),
    focus_level VARCHAR(20),
    day_intensity VARCHAR(20),
    main_focus_today TEXT,
    note TEXT,
    available_time_blocks JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_daily_checkins_user_date UNIQUE (user_id, date)
);

-- daily_reviews
CREATE TABLE IF NOT EXISTS daily_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    date DATE NOT NULL,
    mood VARCHAR(20),
    difficulty_rating INT DEFAULT 0,
    energy_level INT DEFAULT 0,
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

-- log_entries
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

-- xp_transactions
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

-- rewards
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
    claimed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_rewards_user_status ON rewards(user_id, status);

-- reward_redemptions
CREATE TABLE IF NOT EXISTS reward_redemptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    reward_id UUID NOT NULL REFERENCES rewards(id),
    cost INT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reward_redemptions_user_created ON reward_redemptions(user_id, created_at);

-- app_settings
CREATE TABLE IF NOT EXISTS app_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    locale VARCHAR(10) DEFAULT 'en',
    theme VARCHAR(20) DEFAULT 'system',
    daily_quest_limit INT DEFAULT 10,
    notifications_enabled BOOLEAN DEFAULT TRUE,
    quiet_after_time VARCHAR(10) DEFAULT '22:00',
    daily_reminder_time VARCHAR(10),
    weekly_review_day VARCHAR(10),
    timezone VARCHAR(50) DEFAULT 'UTC',
    preferences JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_app_settings_user_id UNIQUE (user_id)
);

-- schema_migrations tracking table (used by golang-migrate)
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE
);
