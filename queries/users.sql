-- name: GetUser :one
SELECT
    id,
    email,
    name,
    created_at,
    updated_at
FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    email,
    name,
    created_at,
    updated_at
FROM users
WHERE email = $1 LIMIT 1;

-- name: ListUsers :many
SELECT
    id,
    email,
    name,
    created_at,
    updated_at
FROM users
ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING id, email, name, created_at, updated_at;

-- name: UpdateUser :exec
UPDATE users
SET name = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
