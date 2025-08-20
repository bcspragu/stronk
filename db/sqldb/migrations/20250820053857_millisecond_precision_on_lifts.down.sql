-- Create table without millisecond precision (original format)
CREATE TABLE lifts_old (
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
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (exercise_id) REFERENCES exercises (id)
);

-- Copy data back, removing millisecond precision
INSERT INTO lifts_old (id, exercise_id, set_type, set_number, reps, weight, day_number, week_number, iteration_number, lift_note, created_at, to_failure)
SELECT id, exercise_id, set_type, set_number, reps, weight, day_number, week_number, iteration_number, lift_note, to_failure,
       substr(created_at, 1, 19)
FROM lifts;

-- Drop new table and rename old one back
DROP TABLE lifts;
ALTER TABLE lifts_old RENAME TO lifts;
