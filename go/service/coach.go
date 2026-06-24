package service

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"naomi.run/coach"
	"naomi.run/database"
)

type coachStore struct {
	q *database.Queries
}

func (s coachStore) AppendMessage(ctx context.Context, userID uuid.UUID, role, content string) (uuid.UUID, error) {
	m, err := s.q.InsertMessage(ctx, database.InsertMessageParams{UserID: userID, Role: role, Content: content})
	if err != nil {
		return uuid.UUID{}, err
	}
	return m.ID, nil
}

func (s coachStore) RecentMessages(ctx context.Context, userID uuid.UUID, limit int) ([]coach.Turn, error) {
	rows, err := s.q.ListRecentMessages(ctx, database.ListRecentMessagesParams{UserID: userID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	turns := make([]coach.Turn, 0, len(rows))
	for _, m := range rows {
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
