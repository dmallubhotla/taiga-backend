-- name: GetUser :one
SELECT
  id,
  email,
  display_name,
  created_at,
  updated_at
FROM
  users
WHERE
  id = $1
LIMIT
  1;

-- -- 
-- really we don't want to select password except specifically for auth.
-- This query is useful for auth only and for nothing else, which should discourage misuse
-- name: SelectEmailPasswordForAuth :one
SELECT
  id,
  email,
  display_name,
  password
FROM
  users
WHERE
  email = $1
LIMIT
  1;

--
-- -- name: ListUsers :many
-- SELECT
--   id,
--   email,
--   name,
--   created_at,
--   updated_at
-- FROM
--   users
-- ORDER BY
--   name;
--
-- name: CreateUser :one
INSERT INTO
  users (email, display_name, password)
VALUES
  ($1, $2, $3)
RETURNING
  id,
  email,
  display_name,
  created_at,
  updated_at;

--
-- -- name: UpdateUser :exec
-- UPDATE users
-- SET
--   name = $2,
--   updated_at = CURRENT_TIMESTAMP
-- WHERE
--   id = $1;
--
-- -- name: DeleteUser :exec
-- DELETE FROM users
-- WHERE
--   id = $1;
