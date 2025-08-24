-- Drop the new routine system tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS lifts_new;
DROP TABLE IF EXISTS routine_sets;
DROP TABLE IF EXISTS routine_movements;
DROP TABLE IF EXISTS routine_days;
DROP TABLE IF EXISTS routine_weeks;
DROP TABLE IF EXISTS routines;