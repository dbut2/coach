-- +goose Up

CREATE TABLE strava_connections (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    athlete_id    BIGINT NOT NULL UNIQUE,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    scope         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user ON sessions (user_id);

-- +goose Down

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS strava_connections;
