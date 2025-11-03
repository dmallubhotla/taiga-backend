-- Drop indexes first
DROP INDEX IF EXISTS idx_users_email;

DROP INDEX IF EXISTS idx_workouts_user_id;

DROP INDEX IF EXISTS idx_workouts_activity_file_id;

DROP INDEX IF EXISTS idx_workouts_start_time;

-- Drop tables table
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS hats;

-- DROP TABLE IF EXISTS user_current_hat;
DROP TABLE IF EXISTS activity_file;

DROP TABLE IF EXISTS workouts;

DROP FUNCTION IF EXISTS trigger_set_timestamp;
