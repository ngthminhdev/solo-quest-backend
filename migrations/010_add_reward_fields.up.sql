-- Add duration_minutes, cooldown_minutes, claim_count columns to rewards table
ALTER TABLE rewards ADD COLUMN IF NOT EXISTS duration_minutes INT;
ALTER TABLE rewards ADD COLUMN IF NOT EXISTS cooldown_minutes INT;
ALTER TABLE rewards ADD COLUMN IF NOT EXISTS claim_count INT;
