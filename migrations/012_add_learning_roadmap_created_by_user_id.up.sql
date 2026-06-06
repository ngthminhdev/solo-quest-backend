-- Add created_by_user_id column to learning_roadmaps table
ALTER TABLE learning_roadmaps
ADD COLUMN IF NOT EXISTS created_by_user_id UUID;

-- Add foreign key constraint
ALTER TABLE learning_roadmaps
ADD CONSTRAINT fk_learning_roadmaps_created_by_user
FOREIGN KEY (created_by_user_id) REFERENCES user_profiles(id) ON DELETE SET NULL;

-- Add index for filtering by creator
CREATE INDEX IF NOT EXISTS idx_learning_roadmaps_created_by_user_id
ON learning_roadmaps(created_by_user_id);
