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
    user_id,
    processor_version,
    sport,
    sub_sport,
    excluded_reason,
    source
  )
VALUES
  (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13
  )
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

-- name: UpdateWorkoutDerived :one
-- Everything reprocessing rebuilds from the activity file, and the corrections it applied over it.
UPDATE workouts
SET
  distance_miles = $3,
  time_seconds = $4,
  speed_mph = $5,
  pace_min_per_mile = $6,
  start_time = $7,
  end_time = $8,
  processor_version = $9,
  sport = $10,
  sub_sport = $11,
  excluded_reason = $12,
  source = $13,
  sport_override = $14,
  user_excluded_reason = $15,
  updated_at = CURRENT_TIMESTAMP
WHERE
  id = $1
  AND user_id = $2
RETURNING
  *;

-- name: ListWorkoutsByUserInWindow :many
-- Starting in [window_from, window_to); callers pass far-off bounds for an open end, so the same SQL runs on SQLite.
SELECT
  *
FROM
  workouts
WHERE
  user_id = sqlc.arg(user_id)
  AND start_time >= sqlc.arg(window_from)
  AND start_time < sqlc.arg(window_to)
ORDER BY
  start_time DESC,
  id DESC
LIMIT
  sqlc.arg(page_limit)
OFFSET
  sqlc.arg(page_offset);

-- name: WorkoutTotalsByUser :many
-- Per sport, for workouts starting in [window_from, window_to) that count; an excluded workout stays out of stats.
SELECT
  sport,
  COUNT(*) AS workouts,
  CAST(
    COALESCE(SUM(distance_miles), 0) AS DOUBLE PRECISION
  ) AS distance_miles,
  CAST(
    COALESCE(SUM(time_seconds), 0) AS DOUBLE PRECISION
  ) AS time_seconds
FROM
  workouts
WHERE
  user_id = sqlc.arg(user_id)
  AND excluded_reason = ''
  AND start_time >= sqlc.arg(window_from)
  AND start_time < sqlc.arg(window_to)
GROUP BY
  sport
ORDER BY
  workouts DESC,
  sport;

-- name: FirstWorkoutStartInWindow :one
-- The earliest start in [window_from, window_to), excluded workouts included since they're still listed; no rows when the window is empty.
SELECT
  start_time
FROM
  workouts
WHERE
  user_id = sqlc.arg(user_id)
  AND start_time >= sqlc.arg(window_from)
  AND start_time < sqlc.arg(window_to)
ORDER BY
  start_time
LIMIT
  1;

-- name: ListCountedWorkoutsInWindow :many
-- What a periodic total needs of each counted workout starting in [window_from, window_to), oldest first; bucketing by local week or month happens in Go, since the two databases share no date functions.
SELECT
  sport,
  start_time,
  distance_miles,
  time_seconds
FROM
  workouts
WHERE
  user_id = sqlc.arg(user_id)
  AND excluded_reason = ''
  AND start_time >= sqlc.arg(window_from)
  AND start_time < sqlc.arg(window_to)
ORDER BY
  start_time,
  id;
