-- name: InsertMessage :one
INSERT INTO messages (user_id, role, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListRecentMessages :many
SELECT * FROM (
    SELECT * FROM messages
    WHERE user_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2
) AS recent
ORDER BY created_at ASC, id ASC;
