-- name: GetActivityFile :one
SELECT
  *
FROM
  activity_file
WHERE
  id = $1
  AND user_id = $2
LIMIT
  1;

-- name: ListActivityFilesByUser :many
SELECT
  *
FROM
  activity_file
WHERE
  user_id = $1
ORDER BY
  timestamp DESC;

-- name: CreateActivityFile :one
INSERT INTO
  activity_file (timestamp, file_repo_hash, user_id)
VALUES
  ($1, $2, $3)
RETURNING
  *;

-- name: UpdateActivityFile :one
UPDATE activity_file
SET
  timestamp = $3,
  file_repo_hash = $4,
  updated_at = CURRENT_TIMESTAMP
WHERE
  id = $1
  AND user_id = $2
RETURNING
  *;

-- name: DeleteActivityFile :exec
DELETE FROM activity_file
WHERE
  id = $1
  AND user_id = $2;
