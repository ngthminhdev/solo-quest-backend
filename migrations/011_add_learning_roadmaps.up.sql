CREATE TABLE IF NOT EXISTS learning_roadmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(50) NOT NULL,
    difficulty VARCHAR(20) NOT NULL DEFAULT 'normal',
    estimated_minutes INT NOT NULL DEFAULT 0,
    total_steps INT NOT NULL DEFAULT 0,
    source VARCHAR(20) NOT NULL DEFAULT 'system',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

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

CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_enabled ON learning_roadmaps(enabled);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_category ON learning_roadmaps(category);
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_source ON learning_roadmaps(source);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_steps_roadmap_id ON learning_roadmap_steps(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_learning_roadmap_steps_order ON learning_roadmap_steps(roadmap_id, order_index);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_user_id ON user_learning_roadmaps(user_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_roadmap_id ON user_learning_roadmaps(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmaps_status ON user_learning_roadmaps(user_id, status);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_user_id ON user_learning_roadmap_step_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_roadmap_id ON user_learning_roadmap_step_progress(roadmap_id);
CREATE INDEX IF NOT EXISTS idx_user_learning_roadmap_step_progress_step_id ON user_learning_roadmap_step_progress(step_id);
