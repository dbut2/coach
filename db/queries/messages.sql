-- name: InsertMessage :one
INSERT INTO messages (user_id, role, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: InsertToolMessage :exec
INSERT INTO messages (user_id, role, content, tool_name, tool_payload)
VALUES ($1, 'tool', '', $2, $3);

-- name: ListRecentMessages :many
SELECT * FROM (
    SELECT * FROM messages
    WHERE user_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2
) AS recent
ORDER BY created_at ASC, id ASC;
