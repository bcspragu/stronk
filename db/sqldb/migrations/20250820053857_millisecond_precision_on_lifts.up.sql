-- Create new table with millisecond precision on created_at
CREATE TABLE lifts_new (
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

-- Copy data from old table, filling in millis with ordering info
INSERT INTO lifts_new (id, exercise_id, set_type, set_number, reps, weight, day_number, week_number, iteration_number, lift_note, to_failure, created_at)
SELECT id, exercise_id, set_type, set_number, reps, weight, day_number, week_number, iteration_number, lift_note, to_failure,
       created_at || '.' || exercise_id || set_number || (reps % 10)
FROM lifts;

-- Drop old table and rename new one
DROP TABLE lifts;
ALTER TABLE lifts_new RENAME TO lifts;
