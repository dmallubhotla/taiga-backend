-- A workout's GPS track resampled for route matching (workouts.Track as JSON), rebuilt by reprocessing; none for a run without GPS.
CREATE TABLE workout_tracks (
  workout_id INTEGER PRIMARY KEY REFERENCES workouts (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id),
  track BYTEA NOT NULL
);

-- Groups of a user's runs on the same route, rebuilt by the detect_routes job; a rebuild keeps the id (and name) of the route a group overlaps most.
CREATE TABLE routes (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users (id),
  name TEXT,
  -- The run whose track stands for the route.
  representative_workout_id INTEGER REFERENCES workouts (id) ON DELETE SET NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_routes_user_id ON routes (user_id);

CREATE TABLE workout_routes (
  workout_id INTEGER PRIMARY KEY REFERENCES workouts (id) ON DELETE CASCADE,
  route_id INTEGER NOT NULL REFERENCES routes (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id)
);

CREATE INDEX idx_workout_routes_route_id ON workout_routes (route_id);
