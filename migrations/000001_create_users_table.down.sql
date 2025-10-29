-- Drop index first
DROP INDEX IF EXISTS idx_users_email;

-- Drop tables table
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS hats;

-- DROP TABLE IF EXISTS user_current_hat;
DROP TABLE IF EXISTS activity_file;

DROP FUNCTION IF EXISTS trigger_set_timestamp;
