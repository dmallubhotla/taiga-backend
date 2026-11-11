-- Duration segments (farthest in 30 minutes, an hour) share the table with distance ones; kind says which way a segment ranks.
ALTER TABLE workout_segments
ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'distance';
