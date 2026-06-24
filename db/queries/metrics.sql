-- name: ListActivitiesByUser :many
SELECT * FROM activities
WHERE user_id = $1 AND start_time >= $2
ORDER BY start_time ASC;

-- name: ListActivityStreamsByUser :many
SELECT s.*
FROM activity_streams s
JOIN activities a ON a.id = s.activity_id
WHERE a.user_id = $1 AND a.start_time >= $2;

-- name: GetActivityStream :one
SELECT * FROM activity_streams WHERE activity_id = $1;

-- name: ListWellnessByUser :many
SELECT * FROM wellness_metrics
WHERE user_id = $1 AND date >= $2
ORDER BY date ASC;
