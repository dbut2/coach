-- name: GetPageViews :one
SELECT page_views FROM myapp.stats LIMIT 1;

-- name: IncrementPageViews :exec
UPDATE myapp.stats SET page_views = page_views + 1 WHERE TRUE;
