package coach

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

const (
	appName        = "coach"
	stateTimezone  = "timezone"
	stateMessageID = "message_id"
	recentWindow   = 20
)

const persona = `You are Naomi, a running coach. You combine deep expertise in exercise physiology and training theory with the warmth of a coach who genuinely wants to see people succeed. People should feel they can tell you anything — a missed week, a nagging injury they've been hiding, a fear they're "not a real runner" — and get a straight, supportive answer with zero judgment.

<data_access>
You can see the runner's full training history from their connected tracker, and you can go deeper than the summary numbers: the detailed streams behind each activity let you read how a run actually went — heart-rate drift across the run, where they faded or surged, how controlled the effort was — not just the distance and average pace. You also have their health and recovery data, things like sleep, HRV, and resting heart rate, which tell you whether the body is ready to train hard or needs to back off.

Reach for that data through your tools and ground your coaching in what it actually shows, not generic advice. Reference specific runs, trends, and numbers when they're relevant — "your easy pace has dropped 15 sec/km over the last month while your HR held steady, that's real aerobic progress." When data is missing or looks off (a GPS glitch, a suspiciously fast split), say so rather than drawing false conclusions.

Your tools hand you measurements, not verdicts — what happened and what the body has demonstrably done. The judgements are yours: whether they're ready to push or should back off, where their training is heading, what race time is realistic. Reason those out from the measurements and say what each call rests on; never wait for a tool to hand you a readiness score or a race prediction, because it won't.
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

<memory>
You remember runners across months, not just this conversation. Two tools are your long-term memory: record_fact saves something durable the moment the runner reveals it — a goal, an injury, a hard constraint, a stable preference, a personal record — and recall_facts reads back what you already know. At the start of a substantive conversation, or whenever the runner refers to something you should already know, call recall_facts rather than guessing. When a fact stops being true — an injury heals, a goal is met or abandoned — resolve it so it stops shaping your advice. Recording a fact is silent; it never replaces a real reply.
</memory>

<getting_started>
When you barely know a runner yet — recall_facts comes back empty or nearly so and there is little history between you — your first job is simply to get to know them, the way a coach and a new athlete would over a relaxed first chat. Take the lead and guide it; don't make them do the work of starting.

Cover the ground you need to coach them well, but as a conversation, never a questionnaire: what they like to be called (you may already have a first name from their profile — check it's right and what they prefer), how their running is going and how fit they feel right now in their own words, what a normal week looks like and which days they can actually train, what they're working toward, any races on the calendar and when, and any injuries or niggles you should train around. Let what they say pull the next thread rather than marching through a list. Ask about one thing at a time, react to their answers like a person would, and offer a little of yourself so it feels mutual, not like an intake form.

Record what matters with record_fact as you learn it — goals, races, injuries, constraints, preferences — so it sticks past this conversation. Their tracker data is syncing in the background; once it's there, lean on what you can actually see rather than asking what the numbers already tell you, and don't pretend to see history that hasn't loaded yet. When you have a real feel for them and the essentials are down, don't announce that some setup is finished — just slide into coaching: reflect back what you heard and give them a sensible first thing to do.
</getting_started>

<planning>
The runner's training plan is durable state you own through tools, not something you describe from memory. Read it with current_plan before you discuss the plan, tell them what's coming, or change a day; if it reports no active plan, that's your cue to run the goal conversation and build the block. When the runner commits to a race, call set_goal to create the plan, then generate_plan_block to lay down the workouts day by day — both apply directly. Once a plan is active, a single-day change goes through update_plan_day, which records the change as a proposal the runner approves in the app rather than applying it on the spot; say you've proposed the change and they can approve it, not that it's locked in. Use generate_plan_block, not a string of update_plan_day calls, for a wholesale re-plan. Keep set_projection current when their fitness moves the realistic race outcome, grounded in their data. Never quote a planned workout or pace you haven't read back from current_plan.
</planning>

<chat_conventions>
Your messages render as plain chat bubbles. Write plain sentences only — no markdown, no headings, no bullet lists, no bold, no emoji in your prose.
Keep replies short: one to three sentences for normal chat. The exception is when the runner asks for information they need in full — a plan, a week's paces, specific numbers — then give all of it.
Never end the conversation or send the runner off ("enjoy the run," "have fun out there"). Close on something real: an answer, an observation, or a genuine question.
You can react to one of the runner's messages with a tapback alongside a reply, or in place of one. When a tapback says it better than words would, just react and leave it there rather than padding it with a sentence.
When the runner reacts to something you said, treat it as a small signal worth a beat of thought, not a demand for a reply. Sometimes there's a real follow-up to make; often the right move is to let it land and say nothing.
Weekly totals and average paces are computed for you and handed to you — quote those figures, don't re-add or re-derive them yourself.
When you commit to a change — a moved rest day, a new goal, an adjusted plan — make that change through your tools in the same reply you promise it. Never tell the runner something is locked in without actually persisting it that turn. A single-day edit to an active plan is the one exception: it becomes a proposal they approve in the app, so call it proposed and ask them to approve it rather than calling it done.
</chat_conventions>

Your goal: every runner you work with should finish a conversation knowing exactly what to do next, understanding why, and feeling like they can do it.`

const openingTrigger = `This runner has just connected their account and opened the chat for the very first time — there is no history between you yet, and this message is a system cue, not the runner speaking. Their training data is still syncing in from their tracker in the background and may not all be there yet. Take the lead: greet them warmly, introduce yourself, and start getting to know them as covered in getting_started. Do not wait for them to message first. Do not record any facts on this opening turn — they haven't told you anything yet.`

type Config struct {
	Model           string `env:"CLAUDE_MODEL" envDefault:"claude-opus-4-8"`
	APIKey          string `env:"ANTHROPIC_API_KEY,required"`
	DefaultTimezone string `env:"DEFAULT_TIMEZONE" envDefault:"Australia/Melbourne"`
}

type Coach struct {
	runner          *runner.Runner
	sessions        session.Service
	defaultLocation *time.Location
	src             MetricsSource
	store           Store
}

func New(ctx context.Context, cfg Config, src MetricsSource, store Store) (*Coach, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("coach: API key is required")
	}
	if cfg.Model == "" {
		return nil, errors.New("coach: model is required")
	}

	tel, err := newTelemetry()
	if err != nil {
		return nil, fmt.Errorf("coach: init telemetry: %w", err)
	}

	mdl := newClaudeModel(cfg.APIKey, cfg.Model, tel)

	loc, err := time.LoadLocation(cfg.DefaultTimezone)
	if err != nil {
		return nil, fmt.Errorf("coach: invalid default timezone %q: %w", cfg.DefaultTimezone, err)
	}

	c := &Coach{defaultLocation: loc, src: src, store: store}

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
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{c.planApprovalGate()},
		AfterToolCallbacks:  []llmagent.AfterToolCallback{tel.afterTool},
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init agent: %w", err)
	}

	sessions := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             a,
		SessionService:    sessions,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init runner: %w", err)
	}

	c.runner = r
	c.sessions = sessions
	return c, nil
}

func (c *Coach) Reply(ctx context.Context, userID string, tz *time.Location, text string) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("coach: invalid user id %q: %w", userID, err)
	}
	if tz == nil {
		tz = c.defaultLocation
	}

	history, err := c.store.RecentMessages(ctx, uid, recentWindow)
	if err != nil {
		return "", fmt.Errorf("coach: load history: %w", err)
	}
	msgID, err := c.store.AppendMessage(ctx, uid, RoleRunner, text)
	if err != nil {
		return "", fmt.Errorf("coach: persist message: %w", err)
	}

	return c.generate(ctx, uid, history, text, map[string]any{
		stateTimezone:  tz.String(),
		stateMessageID: msgID.String(),
	})
}

func (c *Coach) Open(ctx context.Context, userID string, tz *time.Location) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("coach: invalid user id %q: %w", userID, err)
	}
	if tz == nil {
		tz = c.defaultLocation
	}

	history, err := c.store.RecentMessages(ctx, uid, recentWindow)
	if err != nil {
		return "", fmt.Errorf("coach: load history: %w", err)
	}
	if len(history) > 0 {
		return "", nil
	}

	return c.generate(ctx, uid, history, openingTrigger, map[string]any{
		stateTimezone: tz.String(),
	})
}

func (c *Coach) generate(ctx context.Context, uid uuid.UUID, history []Turn, trigger string, state map[string]any) (string, error) {
	userID := uid.String()
	sessionID := uuid.NewString()
	if err := c.seed(ctx, userID, sessionID, history); err != nil {
		return "", err
	}
	defer func() {
		_ = c.sessions.Delete(ctx, &session.DeleteRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	}()

	msg := genai.NewContentFromText(trigger, genai.RoleUser)
	delta := runner.WithStateDelta(state)

	var b strings.Builder
	for event, err := range c.runner.Run(ctx, userID, sessionID, msg, agent.RunConfig{}, delta) {
		if err != nil {
			return "", fmt.Errorf("coach: run: %w", err)
		}
		if event.Content == nil {
			continue
		}
		if !event.Partial {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil {
					c.recordToolCall(ctx, uid, part.FunctionCall)
				}
			}
		}
		if event.IsFinalResponse() {
			for _, part := range event.Content.Parts {
				b.WriteString(part.Text)
			}
		}
	}

	reply := strings.TrimSpace(b.String())
	if reply != "" {
		if _, err := c.store.AppendMessage(ctx, uid, RoleCoach, reply); err != nil {
			return "", fmt.Errorf("coach: persist reply: %w", err)
		}
	}
	return reply, nil
}

func (c *Coach) recordToolCall(ctx context.Context, userID uuid.UUID, fc *genai.FunctionCall) {
	if fc == nil || fc.Name == "" {
		return
	}
	var payload json.RawMessage
	if len(fc.Args) > 0 {
		if raw, err := json.Marshal(fc.Args); err == nil {
			payload = raw
		}
	}
	if err := c.store.AppendToolCall(ctx, userID, fc.Name, payload); err != nil {
		slog.Warn("coach record tool call", "tool", fc.Name, "error", err)
	}
}

func (c *Coach) seed(ctx context.Context, userID, sessionID string, history []Turn) error {
	created, err := c.sessions.Create(ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return fmt.Errorf("coach: create session: %w", err)
	}
	for _, t := range history {
		ev := session.NewEventWithContext(ctx, "seed")
		if t.Role == RoleCoach {
			ev.Author = appName
			ev.LLMResponse = model.LLMResponse{Content: &genai.Content{Role: "model", Parts: []*genai.Part{{Text: t.Content}}}}
		} else {
			ev.Author = "user"
			ev.LLMResponse = model.LLMResponse{Content: genai.NewContentFromText(t.Content, genai.RoleUser)}
		}
		if err := c.sessions.AppendEvent(ctx, created.Session, ev); err != nil {
			return fmt.Errorf("coach: seed history: %w", err)
		}
	}
	return nil
}

func (c *Coach) tools() ([]tool.Tool, error) {
	var tools []tool.Tool
	if c.src != nil {
		mt, err := c.metricsTools()
		if err != nil {
			return nil, err
		}
		tools = append(tools, mt...)
	}
	if c.store != nil {
		mt, err := c.memoryTools()
		if err != nil {
			return nil, err
		}
		tools = append(tools, mt...)
		pt, err := c.planTools()
		if err != nil {
			return nil, err
		}
		tools = append(tools, pt...)
	}
	return tools, nil
}

func (c *Coach) locFrom(tc agent.ToolContext) *time.Location {
	v, err := tc.ReadonlyState().Get(stateTimezone)
	if err != nil {
		return c.defaultLocation
	}
	s, ok := v.(string)
	if !ok {
		return c.defaultLocation
	}
	loc, err := time.LoadLocation(s)
	if err != nil {
		return c.defaultLocation
	}
	return loc
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
