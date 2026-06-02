-- Reverse of 002_fix_daily_checkin_enum_constraints
-- This is a safe no-op. We do NOT convert enum strings back to numeric values
-- because that would lose the semantic meaning of the data.
-- If you need to reverse this migration, restore from a database backup.

-- The enum string values (veryLow, low, medium, high, veryHigh, light, normal, busy, overloaded)
-- are the correct final state. Converting them back to 1-5 integers would be lossy.
