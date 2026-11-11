-- The user's corrections, which reprocessing applies over what the file says: a sport that wins over the file's, and a reason to keep the workout out of best efforts, routes and stats. Empty when not corrected.
ALTER TABLE workouts
ADD COLUMN sport_override VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE workouts
ADD COLUMN user_excluded_reason VARCHAR(64) NOT NULL DEFAULT '';
