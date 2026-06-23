CREATE TABLE activities (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    source text NOT NULL,
    source_id text NOT NULL,
    start_time timestamp with time zone NOT NULL,
    sport_type text NOT NULL,
    raw_summary jsonb NOT NULL,
    ingested_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE activity_streams (
    activity_id uuid NOT NULL,
    time_offset_s integer[],
    hr integer[],
    pace_s_per_km numeric[],
    cadence integer[],
    power_w integer[],
    altitude_m numeric[],
    lat numeric[],
    lng numeric[],
    extra jsonb
);

CREATE TABLE conversation_summaries (
    user_id uuid NOT NULL,
    summary text NOT NULL,
    summarized_through_message_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    role text NOT NULL,
    content text NOT NULL,
    tool_name text,
    tool_payload jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE plan_change_proposals (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    plan_id uuid NOT NULL,
    user_id uuid NOT NULL,
    status text DEFAULT 'proposed'::text NOT NULL,
    rationale text,
    proposed_diff jsonb NOT NULL,
    applied_diff jsonb,
    decided_by text,
    decided_at timestamp with time zone,
    triggering_message_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE planned_workouts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    plan_id uuid NOT NULL,
    user_id uuid NOT NULL,
    scheduled_date date NOT NULL,
    workout_type text,
    description text,
    target_distance_m numeric(10,2),
    target_duration_s integer,
    structure jsonb,
    completed_activity_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    name text,
    goal_fact_id uuid,
    start_date date,
    end_date date,
    meta jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE runner_facts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    value jsonb NOT NULL,
    source_message_id uuid NOT NULL,
    salience smallint DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE sessions (
    id text NOT NULL,
    user_id uuid NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE strava_connections (
    user_id uuid NOT NULL,
    athlete_id bigint NOT NULL,
    access_token text NOT NULL,
    refresh_token text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    display_name text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE wellness_metrics (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    date date NOT NULL,
    metric_key text NOT NULL,
    value_num numeric,
    value_json jsonb,
    source text DEFAULT 'garmin'::text NOT NULL,
    ingested_at timestamp with time zone DEFAULT now() NOT NULL
);
