-- Create users table
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  name VARCHAR(255) NOT NULL,
  password bytea,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create an index on email for faster lookups
CREATE INDEX idx_users_email ON users (email);

CREATE OR REPLACE FUNCTION trigger_set_timestamp () RETURNS TRIGGER AS $set_updated$
BEGIN
	NEW.updated_at = NOW();
	RETURN NEW;
END;
$set_updated$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated BEFORE
UPDATE ON users FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp ();
