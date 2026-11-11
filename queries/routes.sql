-- name: UpsertWorkoutTrack :exec
INSERT INTO
  workout_tracks (workout_id, user_id, track)
VALUES
  ($1, $2, $3)
ON CONFLICT (workout_id) DO UPDATE
SET
  track = excluded.track;

-- name: DeleteWorkoutTrack :exec
DELETE FROM workout_tracks
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: ListWorkoutTracksByUser :many
-- Oldest first, the order route detection assigns runs in; routes only group runs of one sport.
SELECT
  workout_tracks.workout_id,
  workout_tracks.track,
  workouts.sport
FROM
  workout_tracks
  JOIN workouts ON workouts.id = workout_tracks.workout_id
WHERE
  workout_tracks.user_id = $1
ORDER BY
  workouts.start_time,
  workouts.id;

-- name: GetWorkoutTrack :one
SELECT
  track
FROM
  workout_tracks
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: CreateRoute :one
INSERT INTO
  routes (user_id, representative_workout_id)
VALUES
  ($1, $2)
RETURNING
  *;

-- name: SetRouteRepresentative :exec
UPDATE routes
SET
  representative_workout_id = $3
WHERE
  id = $1
  AND user_id = $2;

-- name: RenameRoute :one
UPDATE routes
SET
  name = $3
WHERE
  id = $1
  AND user_id = $2
RETURNING
  *;

-- name: DeleteRoute :exec
DELETE FROM routes
WHERE
  id = $1
  AND user_id = $2;

-- name: GetRoute :one
SELECT
  *
FROM
  routes
WHERE
  id = $1
  AND user_id = $2;

-- name: ListRoutesByUser :many
SELECT
  *
FROM
  routes
WHERE
  user_id = $1
ORDER BY
  id;

-- name: ClearRouteRepresentative :exec
-- SQLite doesn't enforce ON DELETE SET NULL without a per-connection pragma, so a deleted workout clears itself.
UPDATE routes
SET
  representative_workout_id = NULL
WHERE
  representative_workout_id = $1
  AND user_id = $2;

-- name: AddWorkoutRoute :exec
INSERT INTO
  workout_routes (workout_id, route_id, user_id)
VALUES
  ($1, $2, $3);

-- name: DeleteWorkoutRoutesByUser :exec
DELETE FROM workout_routes
WHERE
  user_id = $1;

-- name: DeleteWorkoutRoute :exec
DELETE FROM workout_routes
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: ListWorkoutRoutesByUser :many
SELECT
  *
FROM
  workout_routes
WHERE
  user_id = $1;

-- name: GetWorkoutRouteID :one
SELECT
  route_id
FROM
  workout_routes
WHERE
  workout_id = $1
  AND user_id = $2;

-- name: EnqueueDetectRoutes :exec
-- One pending detection per user (idx_jobs_pending), however many workouts change before it runs.
INSERT INTO
  jobs (user_id, kind, run_at)
VALUES
  ($1, 'detect_routes', $2)
ON CONFLICT DO NOTHING;
