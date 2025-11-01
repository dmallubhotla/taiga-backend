-- Create users table
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT UNIQUE NOT NULL,
  display_name TEXT NOT NULL,
  password BLOB,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create an index on email for faster lookups
CREATE INDEX idx_users_email ON users (email);


CREATE TABLE hats (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT,
  description TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  user_id INTEGER REFERENCES users (id)
);

-- CREATE TABLE user_current_hat (
--   user_id INTEGER PRIMARY KEY,
--   hat_id INTEGER,
--   FOREIGN KEY (user_id, hat_id) REFERENCES hats (user_id, id)
-- );
CREATE TABLE activity_file (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp DATETIME NOT NULL,
  -- TODO make unique
  file_repo_hash TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  user_id INTEGER REFERENCES users (id)
);

CREATE TABLE workouts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  distance_miles REAL,
  time_seconds REAL,
  speed_mph REAL,
  pace_min_per_mile REAL,
  start_time DATETIME,
  end_time DATETIME,
  activity_file_id INTEGER REFERENCES activity_file (id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
  user_id INTEGER REFERENCES users (id) NOT NULL
);

-- Create indexes for workouts table for performance
CREATE INDEX idx_workouts_user_id ON workouts (user_id);

CREATE INDEX idx_workouts_activity_file_id ON workouts (activity_file_id);

CREATE INDEX idx_workouts_start_time ON workouts (start_time);
