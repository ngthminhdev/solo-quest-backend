-- Remove created_by_user_id column from learning_roadmaps table
DROP INDEX IF EXISTS idx_learning_roadmaps_created_by_user_id;

ALTER TABLE learning_roadmaps
DROP CONSTRAINT IF EXISTS fk_learning_roadmaps_created_by_user;

ALTER TABLE learning_roadmaps
DROP COLUMN IF EXISTS created_by_user_id;
