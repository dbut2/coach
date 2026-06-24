package coach

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	active     *Plan
	proposals  int
	planDays   int
	lastDiff   json.RawMessage
	lastReason string
}

func (f *fakeStore) AppendMessage(context.Context, uuid.UUID, string, string) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (f *fakeStore) RecentMessages(context.Context, uuid.UUID, int) ([]Turn, error) { return nil, nil }
func (f *fakeStore) RecordFact(context.Context, uuid.UUID, Fact) error              { return nil }
func (f *fakeStore) ActiveFacts(context.Context, uuid.UUID) ([]Fact, error)         { return nil, nil }
func (f *fakeStore) SetFactStatus(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (f *fakeStore) ActivePlan(context.Context, uuid.UUID) (*Plan, error) { return f.active, nil }
func (f *fakeStore) CreatePlan(context.Context, uuid.UUID, string, time.Time, time.Time) (Plan, error) {
	return Plan{}, nil
}
func (f *fakeStore) UpdatePlan(context.Context, uuid.UUID, uuid.UUID, string, time.Time, time.Time, string) error {
	return nil
}
func (f *fakeStore) UpsertPlanDay(context.Context, uuid.UUID, uuid.UUID, PlanDay) error {
	f.planDays++
	return nil
}
func (f *fakeStore) PlannedWorkouts(context.Context, uuid.UUID, time.Time, time.Time) ([]PlannedWorkout, error) {
	return nil, nil
}
func (f *fakeStore) CreateProposal(_ context.Context, _, _ uuid.UUID, rationale string, diff json.RawMessage, _ uuid.UUID) (uuid.UUID, error) {
	f.proposals++
	f.lastDiff = diff
	f.lastReason = rationale
	return uuid.New(), nil
}

func TestGateProposesWhenPlanActive(t *testing.T) {
	store := &fakeStore{active: &Plan{ID: uuid.New(), Status: "active", Name: "Spring Half"}}
	c := &Coach{store: store, defaultLocation: time.UTC}

	args := map[string]any{"date": "2026-07-10", "workout": "rest", "detail": "easy spin if anything"}
	res, err := c.gateUpdatePlanDay(context.Background(), uuid.New(), args, uuid.Nil)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if res == nil || res["status"] != "proposed" {
		t.Fatalf("expected proposed result, got %v", res)
	}
	if store.proposals != 1 {
		t.Fatalf("expected 1 proposal written, got %d", store.proposals)
	}
	if store.planDays != 0 {
		t.Fatalf("gate must not apply the change directly, got %d upserts", store.planDays)
	}
	var got planDayInput
	if err := json.Unmarshal(store.lastDiff, &got); err != nil {
		t.Fatalf("diff not a planDayInput: %v", err)
	}
	if got.Date != "2026-07-10" || got.Workout != "rest" {
		t.Fatalf("diff round-trip mismatch: %+v", got)
	}
	if store.lastReason != "2026-07-10 → rest" {
		t.Fatalf("unexpected rationale %q", store.lastReason)
	}
}

func TestGateBypassesWhenNoActivePlan(t *testing.T) {
	store := &fakeStore{active: nil}
	c := &Coach{store: store, defaultLocation: time.UTC}

	args := map[string]any{"date": "2026-07-10", "workout": "rest"}
	res, err := c.gateUpdatePlanDay(context.Background(), uuid.New(), args, uuid.Nil)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if res != nil {
		t.Fatalf("expected pass-through (nil), got %v", res)
	}
	if store.proposals != 0 {
		t.Fatalf("expected no proposal, got %d", store.proposals)
	}
}

func TestPlanDayInputToPlanDay(t *testing.T) {
	in := planDayInput{Date: "2026-07-10", Workout: "long run", Detail: "steady", DistanceKm: 18.5, DurationMin: 95}
	pd, err := in.toPlanDay()
	if err != nil {
		t.Fatalf("toPlanDay: %v", err)
	}
	if pd.DistanceM != 18500 {
		t.Fatalf("km→m conversion wrong: %v", pd.DistanceM)
	}
	if pd.DurationS != 95*60 {
		t.Fatalf("min→s conversion wrong: %v", pd.DurationS)
	}
	if !pd.Date.Equal(time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date parse wrong: %v", pd.Date)
	}
}

func TestPlanDayInputRejectsBadDate(t *testing.T) {
	if _, err := (planDayInput{Date: "July 10", Workout: "rest"}).toPlanDay(); err == nil {
		t.Fatal("expected error on malformed date")
	}
}
