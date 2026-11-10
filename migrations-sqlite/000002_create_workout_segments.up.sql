-- Fastest effort per standard distance within a workout, computed when the workout is created from a FIT file.
CREATE TABLE workout_segments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workout_id INTEGER NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id),
  name TEXT NOT NULL,
  distance_meters REAL NOT NULL,
  duration_seconds REAL NOT NULL,
  start_offset_seconds REAL NOT NULL,
  end_offset_seconds REAL NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  UNIQUE (workout_id, name)
);

CREATE INDEX idx_workout_segments_user_name_duration ON workout_segments (user_id, name, duration_seconds);
