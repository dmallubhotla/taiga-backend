-- name: GetHat :one
SELECT
  *
FROM
  hats
WHERE
  id = $1
  AND user_id = $2
LIMIT
  1;

-- name: ListHatsByUser :many
SELECT
  *
FROM
  hats
WHERE
  user_id = $1;

-- name: CreateHat :one
INSERT INTO
  hats (name, description, user_id)
VALUES
  ($1, $2, $3)
RETURNING
  *;
