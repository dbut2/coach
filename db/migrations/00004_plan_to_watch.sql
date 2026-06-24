-- +goose Up

ALTER TABLE planned_workouts ADD COLUMN garmin_workout_id  BIGINT;
ALTER TABLE planned_workouts ADD COLUMN garmin_schedule_id BIGINT;

-- +goose Down

ALTER TABLE planned_workouts DROP COLUMN IF EXISTS garmin_schedule_id;
ALTER TABLE planned_workouts DROP COLUMN IF EXISTS garmin_workout_id;
