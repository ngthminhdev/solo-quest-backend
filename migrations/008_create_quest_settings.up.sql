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
