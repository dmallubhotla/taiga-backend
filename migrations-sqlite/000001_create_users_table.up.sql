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
