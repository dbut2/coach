-- name: GetPlannedWorkout :one
SELECT * FROM planned_workouts WHERE id = $1;

-- name: SetPlannedWorkoutGarmin :exec
UPDATE planned_workouts
SET garmin_workout_id = $2, garmin_schedule_id = $3
WHERE id = $1;
