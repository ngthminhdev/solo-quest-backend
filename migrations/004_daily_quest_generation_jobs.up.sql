-- Async daily quest generation jobs.
-- Tracks the state of a background "generate today's quests" run so the
-- HTTP request can return immediately (202) instead of blocking on a slow
-- AI call. The frontend polls the status endpoint until the job completes.

CREATE TABLE IF NOT EXISTS daily_quest_generation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    source VARCHAR(20),
    fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    ai_error_type VARCHAR(50),
    error_message TEXT,

    generated_count INTEGER NOT NULL DEFAULT 0,
    preserved_count INTEGER NOT NULL DEFAULT 0,
    replaced_pending_count INTEGER NOT NULL DEFAULT 0,

    prefer_ai BOOLEAN NOT NULL DEFAULT TRUE,
    force BOOLEAN NOT NULL DEFAULT FALSE,
    replace_pending_only BOOLEAN NOT NULL DEFAULT TRUE,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uq_quest_gen_jobs_user_date UNIQUE (user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_quest_gen_jobs_user_id ON daily_quest_generation_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_quest_gen_jobs_date ON daily_quest_generation_jobs(date);
CREATE INDEX IF NOT EXISTS idx_quest_gen_jobs_status ON daily_quest_generation_jobs(status);
