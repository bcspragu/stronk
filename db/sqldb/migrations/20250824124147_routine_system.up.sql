-- Create the new routine system tables

-- Routines table - represents a routine loaded into the system
CREATE TABLE routines (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Routine weeks table - represents a week within a routine
CREATE TABLE routine_weeks (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  routine_id INTEGER NOT NULL,
  week_name TEXT NOT NULL,
  optional INTEGER NOT NULL DEFAULT FALSE,
  week_order INTEGER NOT NULL, -- 0-based ordering within the routine
  FOREIGN KEY (routine_id) REFERENCES routines (id)
);

-- Routine days table - represents a day within a routine week
CREATE TABLE routine_days (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  routine_week_id INTEGER NOT NULL,
  day_name TEXT NOT NULL,
  day_order INTEGER NOT NULL, -- 0-based ordering within the week
  FOREIGN KEY (routine_week_id) REFERENCES routine_weeks (id)
);

-- Routine movements table - represents a group of sets (warmup, main, assistance)
CREATE TABLE routine_movements (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  routine_day_id INTEGER NOT NULL,
  exercise_id INTEGER NOT NULL,
  set_type TEXT CHECK( set_type IN ('WARMUP', 'MAIN', 'ASSISTANCE') ) NOT NULL,
  movement_order INTEGER NOT NULL, -- 0-based ordering within the day
  FOREIGN KEY (routine_day_id) REFERENCES routine_days (id),
  FOREIGN KEY (exercise_id) REFERENCES exercises (id)
);

-- Routine sets table - represents individual sets within a movement
CREATE TABLE routine_sets (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  routine_movement_id INTEGER NOT NULL,
  rep_target INTEGER NOT NULL,
  to_failure INTEGER NOT NULL DEFAULT FALSE,
  training_max_percentage INTEGER NOT NULL,
  set_order INTEGER NOT NULL, -- 0-based ordering within the movement
  FOREIGN KEY (routine_movement_id) REFERENCES routine_movements (id)
);

-- New lifts table structure - references routine sets instead of storing all the metadata
CREATE TABLE lifts_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  routine_set_id INTEGER NOT NULL,
  weight TEXT NOT NULL,
  reps INTEGER NOT NULL,
  lift_note TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
  FOREIGN KEY (routine_set_id) REFERENCES routine_sets (id)
);

-- This is important! Basic queries will be quite slow otherwise.
-- I measured this at 370ms vs 7ms (a 50x reduction) on my personal instance.
CREATE INDEX idx_lifts_created_at ON lifts_new (created_at);
