CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  name TEXT UNIQUE NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE exercises (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  name TEXT UNIQUE NOT NULL
);

CREATE TABLE lifts (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  user_id INTEGER NOT NULL,
  exercise_id NOT NULL,
  set_type TEXT CHECK( set_type IN ('WARMUP', 'MAIN', 'ASSISTANCE') ) NOT NULL,
  set_number INTEGER NOT NULL,
  reps INTEGER NOT NULL,
  weight TEXT NOT NULL,
  day_number INTEGER NOT NULL,
  week_number INTEGER NOT NULL,
  iteration_number INTEGER NOT NULL,
  lift_note TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users (id),
  FOREIGN KEY (exercise_id) REFERENCES exercises (id)
);

CREATE TABLE training_maxes (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  user_id INTEGER NOT NULL,
  exercise_id NOT NULL,
  training_max_weight TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users (id),
  FOREIGN KEY (exercise_id) REFERENCES exercises (id)
);
