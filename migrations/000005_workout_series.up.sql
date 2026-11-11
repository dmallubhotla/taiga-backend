-- A workout's downsampled records as JSON columns (workouts.Series), rebuilt by reprocessing.
CREATE TABLE workout_series (
  workout_id INTEGER PRIMARY KEY REFERENCES workouts (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id),
  series BYTEA NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
