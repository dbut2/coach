-- name: InsertRunnerFact :one
INSERT INTO runner_facts (user_id, type, status, value, source_message_id, salience)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListActiveRunnerFacts :many
SELECT * FROM runner_facts
WHERE user_id = $1 AND status = 'active'
ORDER BY salience DESC, updated_at DESC;

-- name: UpdateRunnerFactStatus :exec
UPDATE runner_facts SET status = $2, updated_at = now()
WHERE id = $1 AND user_id = $3;
