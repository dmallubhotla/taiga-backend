-- name: EnqueueStaleWorkouts :execrows
-- Every user's file-backed workouts not built by the current processor; one already queued is skipped by idx_jobs_pending.
INSERT INTO
  jobs (user_id, kind, workout_id, run_at)
SELECT
  user_id,
  'process_workout',
  id,
  $2
FROM
  workouts
WHERE
  activity_file_id IS NOT NULL
  AND processor_version <> $1
ON CONFLICT DO NOTHING;

-- name: EnqueueUserWorkouts :execrows
INSERT INTO
  jobs (user_id, kind, workout_id, run_at)
SELECT
  user_id,
  'process_workout',
  id,
  $2
FROM
  workouts
WHERE
  activity_file_id IS NOT NULL
  AND workouts.user_id = $1
ON CONFLICT DO NOTHING;

-- name: ClaimJob :one
-- The outer state check makes a lost race claim nothing (postgres re-checks it after waiting on the row lock), so no SKIP LOCKED and the same SQL runs on sqlite.
UPDATE jobs
SET
  state = 'running',
  attempts = attempts + 1,
  lease_until = @lease_until,
  updated_at = @now
WHERE
  id = (
    SELECT
      id
    FROM
      jobs
    WHERE
      (
        state = 'queued'
        AND run_at <= @now
      )
      OR (
        state = 'running'
        AND lease_until < @now
      )
    ORDER BY
      run_at,
      id
    LIMIT
      1
  )
  AND (
    state = 'queued'
    OR lease_until < @now
  )
RETURNING
  *;

-- name: FinishJob :exec
UPDATE jobs
SET
  state = 'done',
  lease_until = NULL,
  last_error = NULL,
  updated_at = $2
WHERE
  id = $1;

-- name: RetryJobLater :exec
UPDATE jobs
SET
  state = 'queued',
  run_at = $2,
  lease_until = NULL,
  last_error = $3,
  updated_at = $4
WHERE
  id = $1;

-- name: KillJob :exec
UPDATE jobs
SET
  state = 'dead',
  lease_until = NULL,
  last_error = $2,
  updated_at = $3
WHERE
  id = $1;

-- name: RequeueDeadJob :one
UPDATE jobs
SET
  state = 'queued',
  attempts = 0,
  run_at = $3,
  updated_at = $3
WHERE
  id = $1
  AND user_id = $2
  AND state = 'dead'
RETURNING
  *;

-- name: GetJob :one
SELECT
  *
FROM
  jobs
WHERE
  id = $1
  AND user_id = $2;

-- name: ListJobsByUser :many
SELECT
  *
FROM
  jobs
WHERE
  user_id = $1
ORDER BY
  id DESC
LIMIT
  $2;

-- name: ListJobsByUserAndState :many
SELECT
  *
FROM
  jobs
WHERE
  user_id = $1
  AND state = $2
ORDER BY
  id DESC
LIMIT
  $3;

-- name: DeleteJobsByWorkout :exec
DELETE FROM jobs
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: DeleteDoneJobsBefore :execrows
DELETE FROM jobs
WHERE
  state = 'done'
  AND updated_at < $1;
