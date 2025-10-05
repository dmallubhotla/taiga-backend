-- name: GetUser :one
SELECT
  id,
  email,
  name,
  created_at,
  updated_at
FROM
  users
WHERE
  id = ?
LIMIT
  1;

-- name: GetUserByEmail :one
SELECT
  id,
  email,
  name,
  created_at,
  updated_at
FROM
  users
WHERE
  email = ?
LIMIT
  1;

-- name: ListUsers :many
SELECT
  id,
  email,
  name,
  created_at,
  updated_at
FROM
  users
ORDER BY
  name;

-- name: CreateUser :one
INSERT INTO
  users (email, name)
VALUES
  (?, ?)
RETURNING
  id,
  email,
  name,
  created_at,
  updated_at;

-- name: UpdateUser :exec
UPDATE users
SET
  name = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE
  id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE
  id = ?;
