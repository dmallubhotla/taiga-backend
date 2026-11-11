-- The FIT session's sport and sub-sport (e.g. running / indoor_running); empty for hand-entered workouts or when the file doesn't say.
ALTER TABLE workouts
ADD COLUMN sport VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE workouts
ADD COLUMN sub_sport VARCHAR(32) NOT NULL DEFAULT '';
