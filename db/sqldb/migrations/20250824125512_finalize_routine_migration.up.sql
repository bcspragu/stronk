-- This migration should be run AFTER the data migration is complete
-- It replaces the old lifts table with the new one

-- Drop the old lifts table
DROP TABLE lifts;

-- Rename the new lifts table to replace the old one
ALTER TABLE lifts_new RENAME TO lifts;