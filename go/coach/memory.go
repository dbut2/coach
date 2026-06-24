package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const (
	RoleRunner = "runner"
	RoleCoach  = "coach"

	statusActive = "active"
)

var factTypes = []string{"goal", "injury", "constraint", "preference", "pr"}
var factStatuses = []string{"resolved", "superseded"}

type Turn struct {
	Role    string
	Content string
}

type Fact struct {
	ID              uuid.UUID
	Type            string
	Status          string
	Value           json.RawMessage
	SourceMessageID uuid.UUID
	Salience        int
}

type Store interface {
	AppendMessage(ctx context.Context, userID uuid.UUID, role, content string) (uuid.UUID, error)
	RecentMessages(ctx context.Context, userID uuid.UUID, limit int) ([]Turn, error)
	RecordFact(ctx context.Context, userID uuid.UUID, f Fact) error
	ActiveFacts(ctx context.Context, userID uuid.UUID) ([]Fact, error)
	SetFactStatus(ctx context.Context, userID, factID uuid.UUID, status string) error
}

type factValue struct {
	Text string `json:"text"`
}

func (c *Coach) memoryTools() ([]tool.Tool, error) {
	record, err := functiontool.New(functiontool.Config{
		Name:        "record_fact",
		Description: "Persist a durable fact about the runner so it survives beyond this conversation: a goal, an injury, a hard constraint (days they can't run, terrain, gear), a stable preference, or a personal record. Call this the moment the runner reveals something lasting. Record silently and keep coaching — this is not a substitute for replying.",
	}, func(tc agent.ToolContext, in recordFactInput) (recordFactResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return recordFactResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		if !slices.Contains(factTypes, in.Type) {
			return recordFactResult{}, fmt.Errorf("coach: fact type %q must be one of %v", in.Type, factTypes)
		}
		mid, err := messageID(tc)
		if err != nil {
			return recordFactResult{}, err
		}
		val, err := json.Marshal(factValue{Text: in.Fact})
		if err != nil {
			return recordFactResult{}, err
		}
		f := Fact{
			Type:            in.Type,
			Status:          statusActive,
			Value:           val,
			SourceMessageID: mid,
			Salience:        in.Salience,
		}
		if err := c.store.RecordFact(tc, uid, f); err != nil {
			return recordFactResult{}, fmt.Errorf("coach: record fact: %w", err)
		}
		return recordFactResult{Recorded: true}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	recall, err := functiontool.New(functiontool.Config{
		Name:        "recall_facts",
		Description: "Read back everything you durably know about this runner — their active goals, injuries, constraints, preferences, and personal records. Call this at the start of a substantive conversation, or whenever the runner refers to something you should already know, rather than guessing.",
	}, func(tc agent.ToolContext, _ struct{}) ([]factDTO, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return nil, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		facts, err := c.store.ActiveFacts(tc, uid)
		if err != nil {
			return nil, fmt.Errorf("coach: recall facts: %w", err)
		}
		return toFactDTOs(facts), nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	resolve, err := functiontool.New(functiontool.Config{
		Name:        "resolve_fact",
		Description: "Retire a fact that has stopped being true so it no longer shapes your advice: mark it resolved when an injury heals or a goal is met or abandoned, or superseded when a newer fact replaces it. Use the id from recall_facts.",
	}, func(tc agent.ToolContext, in resolveFactInput) (recordFactResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return recordFactResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		fid, err := uuid.Parse(in.ID)
		if err != nil {
			return recordFactResult{}, fmt.Errorf("coach: invalid fact id %q: %w", in.ID, err)
		}
		if !slices.Contains(factStatuses, in.Status) {
			return recordFactResult{}, fmt.Errorf("coach: status %q must be one of %v", in.Status, factStatuses)
		}
		if err := c.store.SetFactStatus(tc, uid, fid, in.Status); err != nil {
			return recordFactResult{}, fmt.Errorf("coach: resolve fact: %w", err)
		}
		return recordFactResult{Recorded: true}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	return []tool.Tool{record, recall, resolve}, nil
}

func messageID(tc agent.ToolContext) (uuid.UUID, error) {
	v, err := tc.ReadonlyState().Get(stateMessageID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("coach: missing source message id: %w", err)
	}
	s, ok := v.(string)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("coach: source message id has unexpected type %T", v)
	}
	return uuid.Parse(s)
}

func toFactDTOs(facts []Fact) []factDTO {
	out := make([]factDTO, 0, len(facts))
	for _, f := range facts {
		var v factValue
		_ = json.Unmarshal(f.Value, &v)
		out = append(out, factDTO{
			ID:       f.ID.String(),
			Type:     f.Type,
			Fact:     v.Text,
			Salience: f.Salience,
		})
	}
	return out
}

type recordFactInput struct {
	Type     string `json:"type" jsonschema:"one of: goal, injury, constraint, preference, pr"`
	Fact     string `json:"fact" jsonschema:"the fact in plain language, e.g. 'targeting a sub-50 10k in October' or 'left achilles tendinopathy, avoid speedwork'"`
	Salience int    `json:"salience,omitempty" jsonschema:"importance 0-10, higher is surfaced first; goals and injuries are usually high"`
}

type resolveFactInput struct {
	ID     string `json:"id" jsonschema:"the fact id from recall_facts"`
	Status string `json:"status" jsonschema:"resolved (no longer true) or superseded (replaced by a newer fact)"`
}

type recordFactResult struct {
	Recorded bool `json:"recorded"`
}

type factDTO struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Fact     string `json:"fact"`
	Salience int    `json:"salience"`
}
