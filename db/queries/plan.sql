-- name: GetPlannedWorkout :one
SELECT * FROM planned_workouts WHERE id = $1;

-- name: SetPlannedWorkoutGarmin :exec
UPDATE planned_workouts
SET garmin_workout_id = $2, garmin_schedule_id = $3
WHERE id = $1;

-- name: CreatePlan :one
INSERT INTO plans (user_id, status, name, goal_fact_id, start_date, end_date, meta)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetActivePlan :one
SELECT * FROM plans
WHERE user_id = $1 AND status = 'active'
ORDER BY updated_at DESC
LIMIT 1;

-- name: SetPlanStatus :exec
UPDATE plans SET status = $2, updated_at = now()
WHERE id = $1 AND user_id = $3;

-- name: UpdatePlan :exec
UPDATE plans SET name = $2, start_date = $3, end_date = $4, meta = $5, updated_at = now()
WHERE id = $1 AND user_id = $6;

-- name: UpsertPlannedWorkout :one
INSERT INTO planned_workouts (plan_id, user_id, scheduled_date, workout_type, description, target_distance_m, target_duration_s, structure)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (plan_id, scheduled_date) DO UPDATE
SET workout_type = EXCLUDED.workout_type,
    description = EXCLUDED.description,
    target_distance_m = EXCLUDED.target_distance_m,
    target_duration_s = EXCLUDED.target_duration_s,
    structure = EXCLUDED.structure,
    garmin_workout_id = NULL,
    garmin_schedule_id = NULL
RETURNING *;

-- name: ListPlannedWorkoutsInRange :many
SELECT * FROM planned_workouts
WHERE user_id = $1 AND scheduled_date >= $2 AND scheduled_date <= $3
ORDER BY scheduled_date;

-- name: InsertPlanProposal :one
INSERT INTO plan_change_proposals (plan_id, user_id, rationale, proposed_diff, triggering_message_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListProposalsByStatus :many
SELECT * FROM plan_change_proposals
WHERE user_id = $1 AND status = $2
ORDER BY created_at DESC;

-- name: GetProposal :one
SELECT * FROM plan_change_proposals WHERE id = $1;

-- name: DecideProposal :exec
UPDATE plan_change_proposals
SET status = $2, decided_by = $3, decided_at = now(), applied_diff = $4
WHERE id = $1 AND user_id = $5;
