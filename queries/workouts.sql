-- name: GetWorkout :one
SELECT
  *
FROM
  workouts
WHERE
  id = $1
  AND user_id = $2
LIMIT
  1;

-- name: ListWorkoutsByUser :many
SELECT
  *
FROM
  workouts
WHERE
  user_id = $1
ORDER BY
  start_time DESC;

-- name: CreateWorkout :one
INSERT INTO
  workouts (
    distance_miles,
    time_seconds,
    speed_mph,
    pace_min_per_mile,
    start_time,
    end_time,
    activity_file_id,
    user_id
  )
VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
  *;

-- name: UpdateWorkout :one
UPDATE workouts
SET
  distance_miles = $3,
  time_seconds = $4,
  speed_mph = $5,
  pace_min_per_mile = $6,
  start_time = $7,
  end_time = $8,
  activity_file_id = $9,
  updated_at = CURRENT_TIMESTAMP
WHERE
  id = $1
  AND user_id = $2
RETURNING
  *;

-- name: DeleteWorkout :exec
DELETE FROM workouts
WHERE
  id = $1
  AND user_id = $2;

-- name: GetWorkoutsByActivityFile :many
SELECT
  *
FROM
  workouts
WHERE
  activity_file_id = $1
  AND user_id = $2
ORDER BY
  start_time DESC;
