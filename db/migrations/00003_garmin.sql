-- +goose Up

CREATE TABLE garmin_connections (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    oauth_token  TEXT NOT NULL,
    oauth_secret TEXT NOT NULL,
    display_name TEXT NOT NULL,
    full_name    TEXT NOT NULL DEFAULT '',
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_sync    TIMESTAMPTZ
);

-- +goose Down

DROP TABLE IF EXISTS garmin_connections;
