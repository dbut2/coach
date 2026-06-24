package garmin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Step struct {
	Kind      string `json:"kind"`
	DistanceM int    `json:"distance_m,omitempty"`
	DurationS int    `json:"duration_s,omitempty"`
	PaceLow   string `json:"pace_low,omitempty"`
	PaceHigh  string `json:"pace_high,omitempty"`
	Repeat    int    `json:"repeat,omitempty"`
	Steps     []Step `json:"steps,omitempty"`
}

type sportTypeDTO struct {
	SportTypeID  int    `json:"sportTypeId"`
	SportTypeKey string `json:"sportTypeKey"`
}

type stepTypeDTO struct {
	StepTypeID  int    `json:"stepTypeId"`
	StepTypeKey string `json:"stepTypeKey"`
}

type endConditionDTO struct {
	ConditionTypeID  int    `json:"conditionTypeId"`
	ConditionTypeKey string `json:"conditionTypeKey"`
}

type targetTypeDTO struct {
	WorkoutTargetTypeID  int    `json:"workoutTargetTypeId"`
	WorkoutTargetTypeKey string `json:"workoutTargetTypeKey"`
}

type workoutStepDTO struct {
	Type               string           `json:"type"`
	StepOrder          int              `json:"stepOrder"`
	StepType           stepTypeDTO      `json:"stepType"`
	EndCondition       *endConditionDTO `json:"endCondition,omitempty"`
	EndConditionValue  *float64         `json:"endConditionValue,omitempty"`
	TargetType         *targetTypeDTO   `json:"targetType,omitempty"`
	TargetValueOne     *float64         `json:"targetValueOne,omitempty"`
	TargetValueTwo     *float64         `json:"targetValueTwo,omitempty"`
	NumberOfIterations *int             `json:"numberOfIterations,omitempty"`
	SmartRepeat        bool             `json:"smartRepeat"`
	WorkoutSteps       []workoutStepDTO `json:"workoutSteps,omitempty"`
}

type workoutSegmentDTO struct {
	SegmentOrder int              `json:"segmentOrder"`
	SportType    sportTypeDTO     `json:"sportType"`
	WorkoutSteps []workoutStepDTO `json:"workoutSteps"`
}

type WorkoutDTO struct {
	WorkoutID       *int64              `json:"workoutId,omitempty"`
	WorkoutName     string              `json:"workoutName"`
	SportType       sportTypeDTO        `json:"sportType"`
	WorkoutSegments []workoutSegmentDTO `json:"workoutSegments"`
}

var runningSport = sportTypeDTO{SportTypeID: 1, SportTypeKey: "running"}

func stepTypeFor(kind string) stepTypeDTO {
	switch kind {
	case "warmup":
		return stepTypeDTO{1, "warmup"}
	case "cooldown":
		return stepTypeDTO{2, "cooldown"}
	case "recovery":
		return stepTypeDTO{4, "recovery"}
	case "rest":
		return stepTypeDTO{5, "rest"}
	case "repeat":
		return stepTypeDTO{6, "repeat"}
	default:
		return stepTypeDTO{3, "interval"}
	}
}

func BuildWorkout(name string, steps []Step) (*WorkoutDTO, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("garmin: workout has no steps")
	}
	order := 0
	built, err := buildSteps(steps, &order)
	if err != nil {
		return nil, err
	}
	return &WorkoutDTO{
		WorkoutName: name,
		SportType:   runningSport,
		WorkoutSegments: []workoutSegmentDTO{{
			SegmentOrder: 1,
			SportType:    runningSport,
			WorkoutSteps: built,
		}},
	}, nil
}

func buildSteps(steps []Step, order *int) ([]workoutStepDTO, error) {
	var out []workoutStepDTO
	for _, s := range steps {
		*order++
		ord := *order
		if s.Kind == "repeat" {
			n := s.Repeat
			if n < 1 {
				n = 1
			}
			children, err := buildSteps(s.Steps, order)
			if err != nil {
				return nil, err
			}
			iters := n
			out = append(out, workoutStepDTO{
				Type:               "RepeatGroupDTO",
				StepOrder:          ord,
				StepType:           stepTypeFor("repeat"),
				NumberOfIterations: &iters,
				EndCondition:       &endConditionDTO{7, "iterations"},
				WorkoutSteps:       children,
			})
			continue
		}
		step, err := buildExec(s, ord)
		if err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, nil
}

func buildExec(s Step, order int) (workoutStepDTO, error) {
	step := workoutStepDTO{Type: "ExecutableStepDTO", StepOrder: order, StepType: stepTypeFor(s.Kind)}
	switch {
	case s.DistanceM > 0:
		v := float64(s.DistanceM)
		step.EndCondition = &endConditionDTO{3, "distance"}
		step.EndConditionValue = &v
	case s.DurationS > 0:
		v := float64(s.DurationS)
		step.EndCondition = &endConditionDTO{2, "time"}
		step.EndConditionValue = &v
	default:
		step.EndCondition = &endConditionDTO{1, "lap.button"}
	}
	if s.PaceLow != "" || s.PaceHigh != "" {
		lowStr, highStr := s.PaceLow, s.PaceHigh
		if lowStr == "" {
			lowStr = highStr
		}
		if highStr == "" {
			highStr = lowStr
		}
		slow, err := paceToSpeed(lowStr)
		if err != nil {
			return step, err
		}
		fast, err := paceToSpeed(highStr)
		if err != nil {
			return step, err
		}
		one, two := slow, fast
		if one > two {
			one, two = two, one
		}
		step.TargetType = &targetTypeDTO{6, "pace.zone"}
		step.TargetValueOne = &one
		step.TargetValueTwo = &two
	} else {
		step.TargetType = &targetTypeDTO{1, "no.target"}
	}
	return step, nil
}

func paceToSpeed(pace string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(pace), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("garmin: bad pace %q", pace)
	}
	m, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	sec, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("garmin: bad pace %q", pace)
	}
	total := float64(m*60 + sec)
	if total <= 0 {
		return 0, fmt.Errorf("garmin: bad pace %q", pace)
	}
	return 1000.0 / total, nil
}

func (c *Client) CreateWorkout(ctx context.Context, bearer string, w *WorkoutDTO) (int64, error) {
	var resp struct {
		WorkoutID int64 `json:"workoutId"`
	}
	if err := c.apiSend(ctx, bearer, http.MethodPost, "/workout-service/workout", w, &resp); err != nil {
		return 0, err
	}
	if resp.WorkoutID == 0 {
		return 0, fmt.Errorf("garmin: workout created without id")
	}
	return resp.WorkoutID, nil
}

func (c *Client) UpdateWorkout(ctx context.Context, bearer string, id int64, w *WorkoutDTO) error {
	w.WorkoutID = &id
	return c.apiSend(ctx, bearer, http.MethodPut, "/workout-service/workout/"+strconv.FormatInt(id, 10), w, nil)
}

func (c *Client) ScheduleWorkout(ctx context.Context, bearer string, id int64, date string) (int64, error) {
	var resp struct {
		WorkoutScheduleID int64 `json:"workoutScheduleId"`
	}
	body := map[string]string{"date": date}
	if err := c.apiSend(ctx, bearer, http.MethodPost, "/workout-service/schedule/"+strconv.FormatInt(id, 10), body, &resp); err != nil {
		return 0, err
	}
	return resp.WorkoutScheduleID, nil
}

func (c *Client) DeleteWorkout(ctx context.Context, bearer string, id int64) error {
	return c.apiSend(ctx, bearer, http.MethodDelete, "/workout-service/workout/"+strconv.FormatInt(id, 10), nil, nil)
}

func (c *Client) UnscheduleWorkout(ctx context.Context, bearer string, scheduleID int64) error {
	return c.apiSend(ctx, bearer, http.MethodDelete, "/workout-service/schedule/"+strconv.FormatInt(scheduleID, 10), nil, nil)
}

func (c *Client) apiSend(ctx context.Context, bearer, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, apiBase+path, r)
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("User-Agent", apiUserAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("garmin %s %s: status %d: %s", method, path, resp.StatusCode, snippet)
	}
	if out != nil && len(respBody) > 0 && string(respBody) != "null" {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("garmin %s %s: decode: %w", method, path, err)
		}
	}
	return nil
}
