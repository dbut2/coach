package coach

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const planWindowBack = 7 * 24 * time.Hour
const planWindowAhead = 21 * 24 * time.Hour

type Plan struct {
	ID         uuid.UUID
	Status     string
	Name       string
	StartDate  time.Time
	EndDate    time.Time
	Projection string
}

type PlanDay struct {
	Date        time.Time
	WorkoutType string
	Description string
	DistanceM   float64
	DurationS   int
}

type PlannedWorkout struct {
	PlanDay
	Completed bool
}

type planDayInput struct {
	Date        string  `json:"date" jsonschema:"the workout date, YYYY-MM-DD"`
	Workout     string  `json:"workout" jsonschema:"short label, e.g. 'easy run', 'tempo', 'intervals', 'long run', 'rest'"`
	Detail      string  `json:"detail,omitempty" jsonschema:"pace and structure in plain language, e.g. '6k @ 5:10/km then 4x400 @ 4:30'"`
	DistanceKm  float64 `json:"distance_km,omitempty" jsonschema:"target distance in kilometres; omit for rest days"`
	DurationMin int     `json:"duration_min,omitempty" jsonschema:"target duration in minutes; optional"`
}

func (in planDayInput) toPlanDay() (PlanDay, error) {
	d, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return PlanDay{}, fmt.Errorf("coach: invalid date %q (want YYYY-MM-DD): %w", in.Date, err)
	}
	return PlanDay{
		Date:        d,
		WorkoutType: in.Workout,
		Description: in.Detail,
		DistanceM:   in.DistanceKm * 1000,
		DurationS:   in.DurationMin * 60,
	}, nil
}

func decodePlanDay(args map[string]any) (planDayInput, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return planDayInput{}, err
	}
	var in planDayInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return planDayInput{}, err
	}
	return in, nil
}

func (c *Coach) planTools() ([]tool.Tool, error) {
	current, err := functiontool.New(functiontool.Config{
		Name:        "current_plan",
		Description: "Read the runner's active training plan: the goal race, plan dates, current projection, and the planned workouts across the last week and the next three, with what they actually ran. Call this before discussing the plan, changing a day, or telling the runner what's coming up. If there is no active plan, this tells you so — that's your cue to set a goal and build the block.",
	}, func(tc agent.ToolContext, _ struct{}) (currentPlanResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return currentPlanResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		plan, err := c.store.ActivePlan(tc, uid)
		if err != nil {
			return currentPlanResult{}, fmt.Errorf("coach: load plan: %w", err)
		}
		if plan == nil {
			return currentPlanResult{HasPlan: false}, nil
		}
		now := time.Now().In(c.locFrom(tc))
		days, err := c.store.PlannedWorkouts(tc, uid, now.Add(-planWindowBack), now.Add(planWindowAhead))
		if err != nil {
			return currentPlanResult{}, fmt.Errorf("coach: load workouts: %w", err)
		}
		return currentPlanResult{HasPlan: true, Plan: toPlanDTO(plan, days)}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	setGoal, err := functiontool.New(functiontool.Config{
		Name:        "set_goal",
		Description: "Create or update the runner's goal and its training block: the race or objective, the race date, the plan start date, and the block length in weeks. This activates the plan. Call this when the runner commits to a race or changes their target. Setting a goal applies directly — it is not gated. Record the goal as a fact too so it shapes your memory.",
	}, func(tc agent.ToolContext, in setGoalInput) (planActionResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		start, end, err := in.dates(c.locFrom(tc))
		if err != nil {
			return planActionResult{}, err
		}
		plan, err := c.store.ActivePlan(tc, uid)
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: load plan: %w", err)
		}
		if plan == nil {
			if _, err := c.store.CreatePlan(tc, uid, in.Race, start, end); err != nil {
				return planActionResult{}, fmt.Errorf("coach: create plan: %w", err)
			}
			return planActionResult{OK: true}, nil
		}
		if err := c.store.UpdatePlan(tc, uid, plan.ID, in.Race, start, end, plan.Projection); err != nil {
			return planActionResult{}, fmt.Errorf("coach: update plan: %w", err)
		}
		return planActionResult{OK: true}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	generate, err := functiontool.New(functiontool.Config{
		Name:        "generate_plan_block",
		Description: "Write or overwrite a stretch of planned workouts in one call, one entry per date. Use this to build the initial block during onboarding and for major re-plans. It applies directly — it is not gated. Requires an active plan; call set_goal first if there isn't one. To change a single day later, use update_plan_day instead.",
	}, func(tc agent.ToolContext, in generatePlanInput) (planActionResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		plan, err := c.store.ActivePlan(tc, uid)
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: load plan: %w", err)
		}
		if plan == nil {
			return planActionResult{}, fmt.Errorf("coach: no active plan; call set_goal before writing workouts")
		}
		for _, d := range in.Days {
			pd, err := d.toPlanDay()
			if err != nil {
				return planActionResult{}, err
			}
			if err := c.store.UpsertPlanDay(tc, uid, plan.ID, pd); err != nil {
				return planActionResult{}, fmt.Errorf("coach: write workout: %w", err)
			}
		}
		return planActionResult{OK: true, Count: len(in.Days)}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	updateDay, err := functiontool.New(functiontool.Config{
		Name:        "update_plan_day",
		Description: "Change a single day on the active plan: the workout, its detail, distance or duration. While a plan is active, this does not apply immediately — it is recorded as a proposal the runner approves in the app, so tell them you've proposed the change, not that it's locked in. There must be an active plan to edit; build one with set_goal and generate_plan_block first.",
	}, func(tc agent.ToolContext, in planDayInput) (planActionResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		plan, err := c.store.ActivePlan(tc, uid)
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: load plan: %w", err)
		}
		if plan == nil {
			return planActionResult{}, fmt.Errorf("coach: no active plan; call set_goal and generate_plan_block first")
		}
		pd, err := in.toPlanDay()
		if err != nil {
			return planActionResult{}, err
		}
		if err := c.store.UpsertPlanDay(tc, uid, plan.ID, pd); err != nil {
			return planActionResult{}, fmt.Errorf("coach: write workout: %w", err)
		}
		return planActionResult{OK: true}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	setProjection, err := functiontool.New(functiontool.Config{
		Name:        "set_projection",
		Description: "Update the projected race result shown beside the goal, grounded in the runner's current fitness from their data. Applies directly. Requires an active plan.",
	}, func(tc agent.ToolContext, in setProjectionInput) (planActionResult, error) {
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		plan, err := c.store.ActivePlan(tc, uid)
		if err != nil {
			return planActionResult{}, fmt.Errorf("coach: load plan: %w", err)
		}
		if plan == nil {
			return planActionResult{}, fmt.Errorf("coach: no active plan to project")
		}
		if err := c.store.UpdatePlan(tc, uid, plan.ID, plan.Name, plan.StartDate, plan.EndDate, in.Projection); err != nil {
			return planActionResult{}, fmt.Errorf("coach: set projection: %w", err)
		}
		return planActionResult{OK: true}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	return []tool.Tool{current, setGoal, generate, updateDay, setProjection}, nil
}

type setGoalInput struct {
	Race       string `json:"race" jsonschema:"the goal race or objective, e.g. 'Melbourne Half Marathon'"`
	RaceDate   string `json:"race_date,omitempty" jsonschema:"race date YYYY-MM-DD"`
	StartDate  string `json:"start_date,omitempty" jsonschema:"plan start date YYYY-MM-DD; defaults to today"`
	TotalWeeks int    `json:"total_weeks,omitempty" jsonschema:"length of the training block in weeks; used to derive the start date when one isn't given"`
}

func (in setGoalInput) dates(loc *time.Location) (start, end time.Time, err error) {
	if in.RaceDate != "" {
		end, err = time.ParseInLocation("2006-01-02", in.RaceDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("coach: invalid race_date %q: %w", in.RaceDate, err)
		}
	}
	switch {
	case in.StartDate != "":
		start, err = time.ParseInLocation("2006-01-02", in.StartDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("coach: invalid start_date %q: %w", in.StartDate, err)
		}
	case !end.IsZero() && in.TotalWeeks > 0:
		start = end.AddDate(0, 0, -7*in.TotalWeeks)
	default:
		start = time.Now().In(loc).Truncate(24 * time.Hour)
	}
	return start, end, nil
}

type generatePlanInput struct {
	Days []planDayInput `json:"days" jsonschema:"the planned workouts to write, one per date"`
}

type setProjectionInput struct {
	Projection string `json:"projection" jsonschema:"the projected result in plain language, e.g. 'on track for ~1:52 based on current threshold pace'"`
}

type planActionResult struct {
	OK    bool `json:"ok"`
	Count int  `json:"written,omitempty"`
}

type currentPlanResult struct {
	HasPlan bool     `json:"has_plan"`
	Plan    *planDTO `json:"plan,omitempty"`
}

type planDTO struct {
	Name       string       `json:"name,omitempty"`
	RaceDate   string       `json:"race_date,omitempty"`
	StartDate  string       `json:"start_date,omitempty"`
	Projection string       `json:"projection,omitempty"`
	Days       []planDayDTO `json:"days"`
}

type planDayDTO struct {
	Date        string  `json:"date"`
	Weekday     string  `json:"weekday"`
	Workout     string  `json:"workout,omitempty"`
	Detail      string  `json:"detail,omitempty"`
	DistanceKm  float64 `json:"distance_km,omitempty"`
	DurationMin int     `json:"duration_min,omitempty"`
	Completed   bool    `json:"completed"`
}

func toPlanDTO(p *Plan, days []PlannedWorkout) *planDTO {
	dto := &planDTO{Name: p.Name, Projection: p.Projection}
	if !p.EndDate.IsZero() {
		dto.RaceDate = p.EndDate.Format("2006-01-02")
	}
	if !p.StartDate.IsZero() {
		dto.StartDate = p.StartDate.Format("2006-01-02")
	}
	dto.Days = make([]planDayDTO, 0, len(days))
	for _, d := range days {
		row := planDayDTO{
			Date:      d.Date.Format("2006-01-02"),
			Weekday:   d.Date.Format("Mon"),
			Workout:   d.WorkoutType,
			Detail:    d.Description,
			Completed: d.Completed,
		}
		if d.DistanceM > 0 {
			row.DistanceKm = d.DistanceM / 1000
		}
		if d.DurationS > 0 {
			row.DurationMin = d.DurationS / 60
		}
		dto.Days = append(dto.Days, row)
	}
	return dto
}
