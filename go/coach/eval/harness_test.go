//go:build eval

package eval

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"naomi.run/coach"
	"naomi.run/metrics"
)

type config struct {
	APIKey     string `env:"ANTHROPIC_API_KEY"`
	CoachModel string `env:"CLAUDE_MODEL" envDefault:"claude-opus-4-8"`
	JudgeModel string `env:"COACH_EVAL_JUDGE_MODEL" envDefault:"claude-opus-4-8"`
	Timezone   string `env:"DEFAULT_TIMEZONE" envDefault:"Australia/Melbourne"`
}

type scenario struct {
	name     string
	history  []coach.Turn
	facts    []coach.Fact
	plan     *coach.Plan
	workouts []coach.PlannedWorkout
	snapshot metrics.Snapshot

	open     bool
	messages []string

	notes string
}

type outcome struct {
	store   *MockStore
	replies []string
	last    string
}

func liveConfig(t *testing.T) config {
	t.Helper()
	if testing.Short() {
		t.Skip("eval: skipped in -short mode")
	}
	cfg, err := env.ParseAs[config]()
	require.NoError(t, err, "eval: load config")
	if cfg.APIKey == "" {
		t.Skip("eval: ANTHROPIC_API_KEY not set")
	}
	return cfg
}

func (s scenario) run(t *testing.T, cfg config) outcome {
	t.Helper()
	store := s.storeMock(t)
	src := s.sourceMock(t)

	c, err := coach.New(context.Background(), coach.Config{
		Model:           cfg.CoachModel,
		APIKey:          cfg.APIKey,
		DefaultTimezone: cfg.Timezone,
	}, src, store)
	require.NoError(t, err, "eval: build coach")

	uid := uuid.NewString()
	var replies []string
	if s.open {
		reply, err := c.Open(context.Background(), uid, nil)
		require.NoError(t, err, "eval: open")
		replies = append(replies, reply)
	}
	for _, m := range s.messages {
		reply, err := c.Reply(context.Background(), uid, nil, m)
		require.NoErrorf(t, err, "eval: reply %q", m)
		replies = append(replies, reply)
	}

	last := ""
	if len(replies) > 0 {
		last = replies[len(replies)-1]
	}
	return outcome{store: store, replies: replies, last: last}
}

func (s scenario) storeMock(t *testing.T) *MockStore {
	t.Helper()
	m := NewMockStore(t)
	a := mock.Anything
	m.EXPECT().RecentMessages(a, a, a).Return(s.history, nil).Maybe()
	m.EXPECT().AppendMessage(a, a, a, a).Return(uuid.New(), nil).Maybe()
	m.EXPECT().AppendToolCall(a, a, a, a).Return(nil).Maybe()
	m.EXPECT().RecordFact(a, a, a).Return(nil).Maybe()
	m.EXPECT().ActiveFacts(a, a).Return(s.facts, nil).Maybe()
	m.EXPECT().SetFactStatus(a, a, a, a).Return(nil).Maybe()
	m.EXPECT().ActivePlan(a, a).Return(s.plan, nil).Maybe()
	m.EXPECT().CreatePlan(a, a, a, a, a).Return(coach.Plan{}, nil).Maybe()
	m.EXPECT().UpdatePlan(a, a, a, a, a, a, a).Return(nil).Maybe()
	m.EXPECT().UpsertPlanDay(a, a, a, a).Return(nil).Maybe()
	m.EXPECT().PlannedWorkouts(a, a, a, a).Return(s.workouts, nil).Maybe()
	m.EXPECT().CreateProposal(a, a, a, a, a, a).Return(uuid.New(), nil).Maybe()
	return m
}

func (s scenario) sourceMock(t *testing.T) *MockMetricsSource {
	t.Helper()
	m := NewMockMetricsSource(t)
	a := mock.Anything
	m.EXPECT().Snapshot(a, a, a, a).Return(s.snapshot, nil).Maybe()
	return m
}

func (s scenario) transcript(o outcome) string {
	var b strings.Builder
	i := 0
	if s.open {
		b.WriteString("[system: the runner just connected their account and opened the chat for the very first time]\n")
		if i < len(o.replies) {
			fmt.Fprintf(&b, "Coach: %s\n", o.replies[i])
			i++
		}
	}
	for _, m := range s.messages {
		fmt.Fprintf(&b, "Runner: %s\n", m)
		if i < len(o.replies) {
			fmt.Fprintf(&b, "Coach: %s\n", o.replies[i])
			i++
		}
	}
	if s.notes != "" {
		fmt.Fprintf(&b, "\nFIXTURE (ground truth the coach's tools could see):\n%s\n", s.notes)
	}
	return b.String()
}

var (
	markdownHeading = regexp.MustCompile(`(?m)^#{1,6}\s`)
	markdownBullet  = regexp.MustCompile(`(?m)^\s*[-*•]\s`)
	sendoffs        = []string{
		"enjoy the run", "have fun out there", "happy running",
		"have a great run", "enjoy your run", "see you out there",
	}
)

func assertNotEmpty(t *testing.T, text string) {
	t.Helper()
	assert.NotEmpty(t, strings.TrimSpace(text), "eval: coach reply was empty")
}

func assertNoMarkdown(t *testing.T, text string) {
	t.Helper()
	assert.NotContains(t, text, "**", "eval: reply contains markdown bold")
	assert.NotRegexp(t, markdownHeading, text, "eval: reply contains a markdown heading")
	assert.NotRegexp(t, markdownBullet, text, "eval: reply contains a markdown bullet list")
}

func assertNoSendoff(t *testing.T, text string) {
	t.Helper()
	low := strings.ToLower(text)
	for _, s := range sendoffs {
		assert.NotContainsf(t, low, s, "eval: reply ends the conversation with a sign-off (%q)", s)
	}
}

func assertToolCalled(t *testing.T, store *MockStore, name string) {
	t.Helper()
	store.AssertCalled(t, "AppendToolCall", mock.Anything, mock.Anything, name, mock.Anything)
}

func assertToolNotCalled(t *testing.T, store *MockStore, name string) {
	t.Helper()
	store.AssertNotCalled(t, "AppendToolCall", mock.Anything, mock.Anything, name, mock.Anything)
}
