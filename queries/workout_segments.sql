-- name: CreateWorkoutSegment :one
INSERT INTO
  workout_segments (
    workout_id,
    user_id,
    name,
    kind,
    distance_meters,
    duration_seconds,
    start_offset_seconds,
    end_offset_seconds
  )
VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
  *;

-- name: ListSegmentsByWorkout :many
-- Distance segments shortest first, then duration segments shortest first.
SELECT
  *
FROM
  workout_segments
WHERE
  workout_id = $1
  AND user_id = $2
ORDER BY
  kind,
  CASE
    WHEN kind = 'duration' THEN duration_seconds
    ELSE distance_meters
  END;

-- name: ListBestSegmentsByName :many
-- A distance segment ranks by time, a duration segment by distance covered; only workouts of one sport starting in [window_from, window_to) compete.
SELECT
  workout_segments.*,
  workouts.start_time AS workout_start_time
FROM
  workout_segments
  JOIN workouts ON workouts.id = workout_segments.workout_id
WHERE
  workout_segments.user_id = sqlc.arg(user_id)
  AND workout_segments.name = sqlc.arg(name)
  AND workouts.sport = sqlc.arg(sport)
  AND workouts.start_time >= sqlc.arg(window_from)
  AND workouts.start_time < sqlc.arg(window_to)
ORDER BY
  CASE
    WHEN workout_segments.kind = 'duration' THEN - workout_segments.distance_meters
    ELSE workout_segments.duration_seconds
  END
LIMIT
  sqlc.arg(page_limit);

-- name: DeleteSegmentsByWorkout :exec
DELETE FROM workout_segments
WHERE
  workout_id = $1
  AND user_id = $2;
