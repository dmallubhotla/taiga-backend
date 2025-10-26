-- Create users table
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  password bytea,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create an index on email for faster lookups
CREATE INDEX idx_users_email ON users (email);

CREATE TABLE hats (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255),
  description VARCHAR(255),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  user_id int REFERENCES users (id),
  UNIQUE (user_id, id)
);

-- CREATE TABLE user_current_hat (
--   user_id int PRIMARY KEY,
--   hat_id int,
--   FOREIGN KEY (user_id, hat_id) REFERENCES hats (user_id, id)
-- );
CREATE TABLE activity_file (
  id SERIAL PRIMARY KEY,
  timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOT NULL,
  -- TODO make unique
  file_repo_hash VARCHAR(64),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  user_id int REFERENCES users (id),
  UNIQUE (user_id, id)
);

CREATE OR REPLACE FUNCTION trigger_set_timestamp () RETURNS TRIGGER AS $set_updated$
BEGIN
	NEW.updated_at = NOW();
	RETURN NEW;
END;
$set_updated$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated BEFORE
UPDATE ON users FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp ();

CREATE TRIGGER set_updated BEFORE
UPDATE ON hats FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp ();
