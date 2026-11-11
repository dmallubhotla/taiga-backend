DELETE FROM workout_segments
WHERE
  kind = 'duration';

ALTER TABLE workout_segments
DROP COLUMN kind;
