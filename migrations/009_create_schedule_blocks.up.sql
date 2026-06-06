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
