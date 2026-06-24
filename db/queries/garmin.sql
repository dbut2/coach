-- name: GetGarminConnection :one
SELECT * FROM garmin_connections WHERE user_id = $1;

-- name: ListGarminConnections :many
SELECT * FROM garmin_connections;

-- name: UpsertGarminConnection :one
INSERT INTO garmin_connections (user_id, oauth_token, oauth_secret, display_name, full_name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET oauth_token  = EXCLUDED.oauth_token,
    oauth_secret = EXCLUDED.oauth_secret,
    display_name = EXCLUDED.display_name,
    full_name    = EXCLUDED.full_name,
    connected_at = now()
RETURNING *;

-- name: UpdateGarminLastSync :exec
UPDATE garmin_connections SET last_sync = $2 WHERE user_id = $1;

-- name: DeleteGarminConnection :exec
DELETE FROM garmin_connections WHERE user_id = $1;
