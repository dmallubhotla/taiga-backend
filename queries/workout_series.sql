-- name: UpsertWorkoutSeries :exec
INSERT INTO
  workout_series (workout_id, user_id, series)
VALUES
  ($1, $2, $3)
ON CONFLICT (workout_id) DO UPDATE
SET
  series = excluded.series;

-- name: GetWorkoutSeries :one
SELECT
  series
FROM
  workout_series
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: DeleteWorkoutSeries :exec
DELETE FROM workout_series
WHERE
  workout_id = $1
  AND user_id = $2;
