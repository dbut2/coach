-- name: UpsertActivity :one
INSERT INTO activities (user_id, source, source_id, start_time, sport_type, raw_summary)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source, source_id) DO UPDATE
SET start_time  = EXCLUDED.start_time,
    sport_type  = EXCLUDED.sport_type,
    raw_summary = EXCLUDED.raw_summary
RETURNING id;

-- name: DeleteActivity :exec
DELETE FROM activities WHERE source = $1 AND source_id = $2;

-- name: UpsertActivityStream :exec
INSERT INTO activity_streams (activity_id, time_offset_s, hr, pace_s_per_km, cadence, power_w, altitude_m, lat, lng)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (activity_id) DO UPDATE
SET time_offset_s = EXCLUDED.time_offset_s,
    hr            = EXCLUDED.hr,
    pace_s_per_km = EXCLUDED.pace_s_per_km,
    cadence       = EXCLUDED.cadence,
    power_w       = EXCLUDED.power_w,
    altitude_m    = EXCLUDED.altitude_m,
    lat           = EXCLUDED.lat,
    lng           = EXCLUDED.lng;
