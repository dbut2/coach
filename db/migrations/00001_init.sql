-- +goose Up

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE activities (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    source       TEXT NOT NULL,
    source_id    TEXT NOT NULL,
    start_time   TIMESTAMPTZ NOT NULL,
    sport_type   TEXT NOT NULL,
    raw_summary  JSONB NOT NULL,
    ingested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, source_id)
);

CREATE INDEX idx_activities_user_time ON activities (user_id, start_time DESC);

CREATE TABLE activity_streams (
    activity_id   UUID PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    time_offset_s INT[],
    hr            INT[],
    pace_s_per_km NUMERIC[],
    cadence       INT[],
    power_w       INT[],
    altitude_m    NUMERIC[],
    lat           NUMERIC[],
    lng           NUMERIC[],
    extra         JSONB
);

CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    role       TEXT NOT NULL CHECK (role IN ('runner','coach','system','tool')),
    content    TEXT NOT NULL,
    tool_name  TEXT,
    tool_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_user_time ON messages (user_id, created_at);

CREATE TABLE conversation_summaries (
    user_id    UUID PRIMARY KEY REFERENCES users(id),
    summary    TEXT NOT NULL,
    summarized_through_message_id UUID REFERENCES messages(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE runner_facts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id),
    type          TEXT NOT NULL CHECK (type IN ('goal','injury','constraint','preference','pr')),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','resolved','superseded')),
    value         JSONB NOT NULL,
    source_message_id UUID NOT NULL REFERENCES messages(id),
    salience      SMALLINT DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_facts_user_type_status ON runner_facts (user_id, type, status);
CREATE INDEX idx_facts_value ON runner_facts USING GIN (value);

CREATE TABLE plans (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    status       TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','completed','abandoned')),
    name         TEXT,
    goal_fact_id UUID REFERENCES runner_facts(id),
    start_date   DATE,
    end_date     DATE,
    meta         JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_plans_user_status ON plans (user_id, status);

CREATE TABLE planned_workouts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id       UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id),
    scheduled_date DATE NOT NULL,
    workout_type  TEXT,
    description   TEXT,
    target_distance_m NUMERIC(10,2),
    target_duration_s INT,
    structure     JSONB,
    completed_activity_id UUID REFERENCES activities(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_planned_workouts_plan_date ON planned_workouts (plan_id, scheduled_date);
CREATE INDEX idx_planned_workouts_user_date ON planned_workouts (user_id, scheduled_date);

CREATE TABLE plan_change_proposals (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id       UUID NOT NULL REFERENCES plans(id),
    user_id       UUID NOT NULL REFERENCES users(id),
    status        TEXT NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed','approved','rejected','auto_applied')),
    rationale     TEXT,
    proposed_diff JSONB NOT NULL,
    applied_diff  JSONB,
    decided_by    TEXT CHECK (decided_by IN ('runner','agent')),
    decided_at    TIMESTAMPTZ,
    triggering_message_id UUID REFERENCES messages(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_proposals_plan ON plan_change_proposals (plan_id, created_at);
CREATE INDEX idx_proposals_status ON plan_change_proposals (user_id, status);

CREATE TABLE wellness_metrics (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    date        DATE NOT NULL,
    metric_key  TEXT NOT NULL,
    value_num   NUMERIC,
    value_json  JSONB,
    source      TEXT NOT NULL DEFAULT 'garmin',
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, date, metric_key, source)
);

CREATE INDEX idx_wellness_user_date ON wellness_metrics (user_id, date DESC);

-- +goose Down

DROP TABLE IF EXISTS wellness_metrics;
DROP TABLE IF EXISTS plan_change_proposals;
DROP TABLE IF EXISTS planned_workouts;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS runner_facts;
DROP TABLE IF EXISTS conversation_summaries;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS activity_streams;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS users;
