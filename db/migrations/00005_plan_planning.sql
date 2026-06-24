-- +goose Up

CREATE UNIQUE INDEX uq_planned_workouts_plan_date ON planned_workouts (plan_id, scheduled_date);

-- +goose Down

DROP INDEX IF EXISTS uq_planned_workouts_plan_date;
