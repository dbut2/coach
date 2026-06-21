-- +goose Up
CREATE SCHEMA IF NOT EXISTS myapp;

CREATE TABLE myapp.stats (
    page_views INT NOT NULL
);

INSERT INTO myapp.stats (page_views) VALUES (0);

-- +goose Down
DROP TABLE myapp.stats;
DROP SCHEMA IF EXISTS myapp;
