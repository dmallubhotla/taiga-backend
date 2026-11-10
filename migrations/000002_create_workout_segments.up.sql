-- Fastest effort per standard distance within a workout, computed when the workout is created from a FIT file.
CREATE TABLE workout_segments (
  id SERIAL PRIMARY KEY,
  workout_id INTEGER NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id),
  name VARCHAR(32) NOT NULL,
  distance_meters DOUBLE PRECISION NOT NULL,
  duration_seconds DOUBLE PRECISION NOT NULL,
  start_offset_seconds DOUBLE PRECISION NOT NULL,
  end_offset_seconds DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  UNIQUE (workout_id, name)
);

CREATE INDEX idx_workout_segments_user_name_duration ON workout_segments (user_id, name, duration_seconds);
