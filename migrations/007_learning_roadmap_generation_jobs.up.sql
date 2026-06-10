-- Async AI learning roadmap generation jobs.
-- Prevents POST /api/learning-roadmaps/generate from blocking on slow AI calls
-- and gives the frontend a stable job_id to poll.

CREATE TABLE IF NOT EXISTS learning_roadmap_generation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_profiles(id),
    request_hash VARCHAR(64) NOT NULL,
    preferences JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    roadmap_id UUID REFERENCES learning_roadmaps(id),

    error_type VARCHAR(50),
    error_message TEXT,
    generated_step_count INTEGER NOT NULL DEFAULT 0,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uq_learning_roadmap_gen_jobs_user_hash UNIQUE (user_id, request_hash)
);

CREATE INDEX IF NOT EXISTS idx_learning_roadmap_gen_jobs_user_id ON learning_roadmap_generation_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_gen_jobs_status ON learning_roadmap_generation_jobs(status);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_gen_jobs_roadmap_id ON learning_roadmap_generation_jobs(roadmap_id);
