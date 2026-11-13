-- name: GetIdentityUser :one
SELECT
  users.id,
  users.email,
  users.display_name
FROM
  user_identities
  JOIN users ON users.id = user_identities.user_id
WHERE
  user_identities.issuer = $1
  AND user_identities.subject = $2;

-- name: CreateIdentity :one
INSERT INTO
  user_identities (user_id, issuer, subject, email_at_link)
VALUES
  ($1, $2, $3, $4)
RETURNING
  *;

-- name: ListIdentitiesByUser :many
SELECT
  *
FROM
  user_identities
WHERE
  user_id = $1
ORDER BY
  created_at,
  id;

-- name: DeleteIdentity :execrows
DELETE FROM user_identities
WHERE
  id = $1
  AND user_id = $2;
