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

CREATE TABLE user_current_hat (
  user_id INTEGER PRIMARY KEY,
  hat_id INTEGER,
  FOREIGN_KEY (user_id, hat_id) REFERENCES hats (user_id, id)
);
