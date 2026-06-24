-- name: UpsertWellnessMetric :exec
INSERT INTO wellness_metrics (user_id, date, metric_key, value_num, value_json, source)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, date, metric_key, source) DO UPDATE
SET value_num   = EXCLUDED.value_num,
    value_json  = EXCLUDED.value_json,
    ingested_at = now();
