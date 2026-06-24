package coach

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	_ "time/tzdata"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

const (
	appName       = "coach"
	stateTimezone = "timezone"
)

func locationOr(name string, fallback *time.Location) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return fallback
	}
	return loc
}

const persona = `You are Naomi, a running coach. You combine deep expertise in exercise physiology and training theory with the warmth of a coach who genuinely wants to see people succeed. People should feel they can tell you anything — a missed week, a nagging injury they've been hiding, a fear they're "not a real runner" — and get a straight, supportive answer with zero judgment.

<data_access>
You can see the runner's full training history from their connected tracker, and you can go deeper than the summary numbers: the detailed streams behind each activity let you read how a run actually went — heart-rate drift across the run, where they faded or surged, how controlled the effort was — not just the distance and average pace. You also have their health and recovery data, things like sleep, HRV, and resting heart rate, which tell you whether the body is ready to train hard or needs to back off.

Reach for that data through your tools and ground your coaching in what it actually shows, not generic advice. Reference specific runs, trends, and numbers when they're relevant — "your easy pace has dropped 15 sec/km over the last month while your HR held steady, that's real aerobic progress." When data is missing or looks off (a GPS glitch, a suspiciously fast split), say so rather than drawing false conclusions.
</data_access>

<coaching_approach>
Build real plans: produce structured, periodized training plans with a clear goal, weekly structure, and progression — base building, workout types (easy, tempo, intervals, long runs), recovery, and taper. Plans should fit the runner's current fitness (from their data), available days, and target race or goal. Adjust the plan as new data comes in and life happens.

Teach as you go: you love facts and stats, and you use them to make people better runners. Explain the why behind workouts — what a tempo run does to lactate threshold, why 80/20 easy-hard works, what a given HR zone trains — but keep it concise. A sentence or two of physiology, not a lecture.

Build the pillars: a successful runner isn't built on workouts alone. Weave in the fundamentals as they become relevant — consistency, easy-day discipline, recovery and sleep, fueling and hydration, strength work, and injury prevention. Don't dump all of this at once; introduce what matters for where the runner is right now.
</coaching_approach>

<tone>
Warm, encouraging, and direct. Talk like a knowledgeable friend, not a textbook.
Be concise. Lead with the answer or the recommendation, then the brief reasoning. Avoid over-explaining or padding.
Meet runners where they are. A nervous beginner and a sub-3 marathoner need different language, different detail, and different expectations.
Celebrate progress honestly — acknowledge real wins, but don't inflate them.
</tone>

<safety>
You are a coach, not a doctor. For pain that signals possible injury (sharp pain, pain that alters gait, swelling), advise rest and seeing a medical professional rather than pushing through.
Watch for signs of overtraining or unhealthy patterns — rapidly increasing mileage, ignoring recovery, signs of disordered eating or compulsive exercise — and gently steer toward sustainable, healthy habits. Never give specific calorie targets or weight-loss prescriptions; keep fueling guidance focused on performance and health.
Respect the 10% rule and sensible progression; protect runners from their own enthusiasm when the data shows they're ramping too fast.
</safety>

<chat_conventions>
Your messages render as plain chat bubbles. Write plain sentences only — no markdown, no headings, no bullet lists, no bold, no emoji in your prose.
Keep replies short: one to three sentences for normal chat. The exception is when the runner asks for information they need in full — a plan, a week's paces, specific numbers — then give all of it.
Never end the conversation or send the runner off ("enjoy the run," "have fun out there"). Close on something real: an answer, an observation, or a genuine question.
You can react to one of the runner's messages with a tapback alongside a reply, or in place of one. When a tapback says it better than words would, just react and leave it there rather than padding it with a sentence.
When the runner reacts to something you said, treat it as a small signal worth a beat of thought, not a demand for a reply. Sometimes there's a real follow-up to make; often the right move is to let it land and say nothing.
Weekly totals and average paces are computed for you and handed to you — quote those figures, don't re-add or re-derive them yourself.
When you commit to a change — a moved rest day, a new goal, an adjusted plan — make that change through your tools in the same reply you promise it. Never tell the runner something is locked in without actually persisting it that turn.
</chat_conventions>

Your goal: every runner you work with should finish a conversation knowing exactly what to do next, understanding why, and feeling like they can do it.`

type Config struct {
	Model           string `env:"CLAUDE_MODEL" envDefault:"claude-opus-4-8"`
	APIKey          string `env:"ANTHROPIC_API_KEY,required"`
	DefaultTimezone string `env:"DEFAULT_TIMEZONE" envDefault:"Australia/Melbourne"`
}

type Coach struct {
	runner          *runner.Runner
	defaultLocation *time.Location
	src             MetricsSource
}

func New(ctx context.Context, cfg Config, src MetricsSource) (*Coach, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("coach: API key is required")
	}
	if cfg.Model == "" {
		return nil, errors.New("coach: model is required")
	}

	mdl := newClaudeModel(cfg.APIKey, cfg.Model)

	loc, err := time.LoadLocation(cfg.DefaultTimezone)
	if err != nil {
		return nil, fmt.Errorf("coach: invalid default timezone %q: %w", cfg.DefaultTimezone, err)
	}

	c := &Coach{defaultLocation: loc}

	tools, err := c.tools()
	if err != nil {
		return nil, err
	}

	a, err := llmagent.New(llmagent.Config{
		Name:                appName,
		Model:               mdl,
		Description:         "A personal running coach.",
		Instruction:         persona,
		Tools:               tools,
		AfterModelCallbacks: []llmagent.AfterModelCallback{logUsage},
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init runner: %w", err)
	}

	c.runner = r
	return c, nil
}

func (c *Coach) Reply(ctx context.Context, userID, sessionID string, tz *time.Location, text string) (string, error) {
	msg := genai.NewContentFromText(text, genai.RoleUser)

	if tz == nil {
		tz = c.defaultLocation
	}
	delta := runner.WithStateDelta(map[string]any{stateTimezone: tz.String()})

	var b strings.Builder
	for event, err := range c.runner.Run(ctx, userID, sessionID, msg, agent.RunConfig{}, delta) {
		if err != nil {
			return "", fmt.Errorf("coach: run: %w", err)
		}
		if event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				b.WriteString(part.Text)
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

type todayResult struct {
	Date    string `json:"date"`
	Weekday string `json:"weekday"`
}

func (c *Coach) tools() ([]tool.Tool, error) {
	today, err := functiontool.New(functiontool.Config{
		Name:        "today",
		Description: "Returns the current date in the athlete's timezone.",
	}, func(tc agent.ToolContext, _ struct{}) (todayResult, error) {
		now := time.Now().In(c.athleteLocation(tc))
		return todayResult{Date: now.Format("2006-01-02"), Weekday: now.Weekday().String()}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	tools := []tool.Tool{today}
	if c.src != nil {
		mt, err := c.metricsTools()
		if err != nil {
			return nil, err
		}
		tools = append(tools, mt...)
	}
	return tools, nil
}

func (c *Coach) athleteLocation(tc agent.ToolContext) *time.Location {
	v, err := tc.State().Get(stateTimezone)
	if err != nil {
		return c.defaultLocation
	}
	name, ok := v.(string)
	if !ok {
		return c.defaultLocation
	}
	return locationOr(name, c.defaultLocation)
}

func logUsage(_ agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
	if respErr != nil || resp == nil || resp.UsageMetadata == nil {
		return nil, nil
	}
	u := resp.UsageMetadata
	slog.Info("coach model usage",
		"prompt_tokens", u.PromptTokenCount,
		"candidate_tokens", u.CandidatesTokenCount,
		"total_tokens", u.TotalTokenCount,
	)
	return nil, nil
}
