-- Reverse the finalization - restore the old lifts table structure
-- Note: This will lose data in the new format

-- Rename current lifts table back to lifts_new
ALTER TABLE lifts RENAME TO lifts_new;

-- Recreate the old lifts table structure
CREATE TABLE lifts (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  exercise_id NOT NULL,
  set_type TEXT CHECK( set_type IN ('WARMUP', 'MAIN', 'ASSISTANCE') ) NOT NULL,
  set_number INTEGER NOT NULL,
  reps INTEGER NOT NULL,
  weight TEXT NOT NULL,
  day_number INTEGER NOT NULL,
  week_number INTEGER NOT NULL,
  iteration_number INTEGER NOT NULL,
  lift_note TEXT,
  to_failure INTEGER NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
  FOREIGN KEY (exercise_id) REFERENCES exercises (id)
);