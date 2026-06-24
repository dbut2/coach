package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"naomi.run/coach"
	"naomi.run/database"
	"naomi.run/web"
)

type coachStore struct {
	q         *database.Queries
	hub       *hub
	loc       *time.Location
	coachName string
}

func (s coachStore) AppendMessage(ctx context.Context, userID uuid.UUID, role, content string) (uuid.UUID, error) {
	m, err := s.q.InsertMessage(ctx, database.InsertMessageParams{UserID: userID, Role: role, Content: content})
	if err != nil {
		return uuid.UUID{}, err
	}
	if role == coach.RoleCoach && s.hub != nil {
		s.hub.broadcast(userID.String(), "message", renderHTML(web.Fragment(web.Message{
			Role:    web.RoleAssistant,
			Content: content,
			Time:    m.CreatedAt.In(s.loc).Format("3:04 PM"),
		})))
	}
	return m.ID, nil
}

func (s coachStore) AppendToolCall(ctx context.Context, userID uuid.UUID, name string, payload json.RawMessage) error {
	if err := s.q.InsertToolMessage(ctx, database.InsertToolMessageParams{
		UserID:      userID,
		ToolName:    sql.NullString{String: name, Valid: name != ""},
		ToolPayload: pqtype.NullRawMessage{RawMessage: payload, Valid: len(payload) > 0},
	}); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.broadcast(userID.String(), "tool", renderHTML(web.Fragment(web.Message{
			Role:    web.RoleTool,
			Tool:    name,
			Content: compactJSON(payload),
		})))
	}
	return nil
}

func compactJSON(p json.RawMessage) string {
	if len(p) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, p); err != nil {
		return string(p)
	}
	return buf.String()
}

func (s coachStore) RecentMessages(ctx context.Context, userID uuid.UUID, limit int) ([]coach.Turn, error) {
	rows, err := s.q.ListRecentMessages(ctx, database.ListRecentMessagesParams{UserID: userID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	turns := make([]coach.Turn, 0, len(rows))
	for _, m := range rows {
		if m.Role == coach.RoleTool {
			continue
		}
		turns = append(turns, coach.Turn{Role: m.Role, Content: m.Content})
	}
	return turns, nil
}

func (s coachStore) RecordFact(ctx context.Context, userID uuid.UUID, f coach.Fact) error {
	_, err := s.q.InsertRunnerFact(ctx, database.InsertRunnerFactParams{
		UserID:          userID,
		Type:            f.Type,
		Status:          f.Status,
		Value:           f.Value,
		SourceMessageID: f.SourceMessageID,
		Salience:        sql.NullInt16{Int16: int16(f.Salience), Valid: true},
	})
	return err
}

func (s coachStore) ActiveFacts(ctx context.Context, userID uuid.UUID) ([]coach.Fact, error) {
	rows, err := s.q.ListActiveRunnerFacts(ctx, userID)
	if err != nil {
		return nil, err
	}
	facts := make([]coach.Fact, 0, len(rows))
	for _, r := range rows {
		facts = append(facts, coach.Fact{
			ID:              r.ID,
			Type:            r.Type,
			Status:          r.Status,
			Value:           r.Value,
			SourceMessageID: r.SourceMessageID,
			Salience:        int(r.Salience.Int16),
		})
	}
	return facts, nil
}

func (s coachStore) SetFactStatus(ctx context.Context, userID, factID uuid.UUID, status string) error {
	return s.q.UpdateRunnerFactStatus(ctx, database.UpdateRunnerFactStatusParams{
		ID:     factID,
		Status: status,
		UserID: userID,
	})
}

type planMeta struct {
	Projection string `json:"projection,omitempty"`
}

func toCoachPlan(p database.Plan) *coach.Plan {
	out := &coach.Plan{
		ID:     p.ID,
		Status: p.Status,
		Name:   p.Name.String,
	}
	if p.StartDate.Valid {
		out.StartDate = p.StartDate.Time
	}
	if p.EndDate.Valid {
		out.EndDate = p.EndDate.Time
	}
	if p.Meta.Valid {
		var m planMeta
		if json.Unmarshal(p.Meta.RawMessage, &m) == nil {
			out.Projection = m.Projection
		}
	}
	return out
}

func (s coachStore) ActivePlan(ctx context.Context, userID uuid.UUID) (*coach.Plan, error) {
	p, err := s.q.GetActivePlan(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toCoachPlan(p), nil
}

func (s coachStore) CreatePlan(ctx context.Context, userID uuid.UUID, name string, start, end time.Time) (coach.Plan, error) {
	p, err := s.q.CreatePlan(ctx, database.CreatePlanParams{
		UserID:    userID,
		Status:    "active",
		Name:      sql.NullString{String: name, Valid: name != ""},
		StartDate: nullDate(start),
		EndDate:   nullDate(end),
	})
	if err != nil {
		return coach.Plan{}, err
	}
	return *toCoachPlan(p), nil
}

func (s coachStore) UpdatePlan(ctx context.Context, userID, planID uuid.UUID, name string, start, end time.Time, projection string) error {
	var meta pqtype.NullRawMessage
	if projection != "" {
		raw, err := json.Marshal(planMeta{Projection: projection})
		if err != nil {
			return err
		}
		meta = pqtype.NullRawMessage{RawMessage: raw, Valid: true}
	}
	return s.q.UpdatePlan(ctx, database.UpdatePlanParams{
		ID:        planID,
		UserID:    userID,
		Name:      sql.NullString{String: name, Valid: name != ""},
		StartDate: nullDate(start),
		EndDate:   nullDate(end),
		Meta:      meta,
	})
}

func (s coachStore) UpsertPlanDay(ctx context.Context, userID, planID uuid.UUID, d coach.PlanDay) error {
	_, err := s.q.UpsertPlannedWorkout(ctx, planDayParams(planID, userID, d))
	return err
}

func (s coachStore) PlannedWorkouts(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]coach.PlannedWorkout, error) {
	rows, err := s.q.ListPlannedWorkoutsInRange(ctx, database.ListPlannedWorkoutsInRangeParams{
		UserID:          userID,
		ScheduledDate:   from,
		ScheduledDate_2: to,
	})
	if err != nil {
		return nil, err
	}
	out := make([]coach.PlannedWorkout, 0, len(rows))
	for _, r := range rows {
		out = append(out, coach.PlannedWorkout{
			PlanDay:   toCoachPlanDay(r),
			Completed: r.CompletedActivityID.Valid,
		})
	}
	return out, nil
}

func (s coachStore) CreateProposal(ctx context.Context, userID, planID uuid.UUID, rationale string, diff json.RawMessage, sourceMessageID uuid.UUID) (uuid.UUID, error) {
	p, err := s.q.InsertPlanProposal(ctx, database.InsertPlanProposalParams{
		PlanID:              planID,
		UserID:              userID,
		Rationale:           sql.NullString{String: rationale, Valid: rationale != ""},
		ProposedDiff:        diff,
		TriggeringMessageID: uuid.NullUUID{UUID: sourceMessageID, Valid: sourceMessageID != uuid.Nil},
	})
	if err != nil {
		return uuid.Nil, err
	}
	return p.ID, nil
}

func nullDate(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func planDayParams(planID, userID uuid.UUID, d coach.PlanDay) database.UpsertPlannedWorkoutParams {
	p := database.UpsertPlannedWorkoutParams{
		PlanID:        planID,
		UserID:        userID,
		ScheduledDate: d.Date,
		WorkoutType:   sql.NullString{String: d.WorkoutType, Valid: d.WorkoutType != ""},
		Description:   sql.NullString{String: d.Description, Valid: d.Description != ""},
	}
	if d.DistanceM > 0 {
		p.TargetDistanceM = sql.NullString{String: strconv.FormatFloat(d.DistanceM, 'f', 2, 64), Valid: true}
	}
	if d.DurationS > 0 {
		p.TargetDurationS = sql.NullInt32{Int32: int32(d.DurationS), Valid: true}
	}
	return p
}

func toCoachPlanDay(r database.PlannedWorkout) coach.PlanDay {
	d := coach.PlanDay{
		Date:        r.ScheduledDate,
		WorkoutType: r.WorkoutType.String,
		Description: r.Description.String,
	}
	if r.TargetDistanceM.Valid {
		if m, err := strconv.ParseFloat(r.TargetDistanceM.String, 64); err == nil {
			d.DistanceM = m
		}
	}
	if r.TargetDurationS.Valid {
		d.DurationS = int(r.TargetDurationS.Int32)
	}
	return d
}
