-- name: GetStravaConnectionByAthleteID :one
SELECT * FROM strava_connections WHERE athlete_id = $1;

-- name: GetStravaConnectionByUserID :one
SELECT * FROM strava_connections WHERE user_id = $1;

-- name: CreateUser :one
INSERT INTO users (display_name) VALUES ($1) RETURNING *;

-- name: CreateStravaConnection :one
INSERT INTO strava_connections (user_id, athlete_id, access_token, refresh_token, expires_at, scope)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateStravaTokens :exec
UPDATE strava_connections
SET access_token = $2, refresh_token = $3, expires_at = $4, scope = $5, updated_at = now()
WHERE user_id = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: GetSessionUser :one
SELECT sqlc.embed(users) FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.id = $1 AND sessions.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
