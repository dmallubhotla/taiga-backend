-- name: CreateWorkoutSegment :one
INSERT INTO
  workout_segments (
    workout_id,
    user_id,
    name,
    distance_meters,
    duration_seconds,
    start_offset_seconds,
    end_offset_seconds
  )
VALUES
  ($1, $2, $3, $4, $5, $6, $7)
RETURNING
  *;

-- name: ListSegmentsByWorkout :many
SELECT
  *
FROM
  workout_segments
WHERE
  workout_id = $1
  AND user_id = $2
ORDER BY
  distance_meters;

-- name: ListBestSegmentsByName :many
SELECT
  workout_segments.*,
  workouts.start_time AS workout_start_time
FROM
  workout_segments
  JOIN workouts ON workouts.id = workout_segments.workout_id
WHERE
  workout_segments.user_id = $1
  AND workout_segments.name = $2
ORDER BY
  workout_segments.duration_seconds
LIMIT
  $3;

-- name: DeleteSegmentsByWorkout :exec
DELETE FROM workout_segments
WHERE
  workout_id = $1
  AND user_id = $2;
