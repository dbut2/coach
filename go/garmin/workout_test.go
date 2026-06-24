package garmin

import (
	"math"
	"testing"
)

func TestBuildWorkoutStructure(t *testing.T) {
	steps := []Step{
		{Kind: "warmup", DistanceM: 1000},
		{Kind: "repeat", Repeat: 6, Steps: []Step{
			{Kind: "interval", DistanceM: 800, PaceLow: "4:50", PaceHigh: "4:40"},
			{Kind: "recovery", DurationS: 90},
		}},
		{Kind: "cooldown", DistanceM: 1000},
	}
	w, err := BuildWorkout("Intervals 6x800", steps)
	if err != nil {
		t.Fatal(err)
	}
	if w.SportType.SportTypeKey != "running" || w.WorkoutName != "Intervals 6x800" {
		t.Fatalf("unexpected header: %+v", w.SportType)
	}
	top := w.WorkoutSegments[0].WorkoutSteps
	if len(top) != 3 {
		t.Fatalf("want 3 top steps, got %d", len(top))
	}

	wu := top[0]
	if wu.StepOrder != 1 || wu.StepType.StepTypeKey != "warmup" {
		t.Errorf("warmup: order %d type %s", wu.StepOrder, wu.StepType.StepTypeKey)
	}
	if wu.EndCondition.ConditionTypeKey != "distance" || *wu.EndConditionValue != 1000 {
		t.Errorf("warmup end: %+v", wu.EndCondition)
	}
	if wu.TargetType.WorkoutTargetTypeKey != "no.target" {
		t.Errorf("warmup target: %s", wu.TargetType.WorkoutTargetTypeKey)
	}

	rg := top[1]
	if rg.Type != "RepeatGroupDTO" || rg.StepOrder != 2 || rg.NumberOfIterations == nil || *rg.NumberOfIterations != 6 {
		t.Fatalf("repeat group: %+v", rg)
	}
	if len(rg.WorkoutSteps) != 2 {
		t.Fatalf("want 2 children, got %d", len(rg.WorkoutSteps))
	}

	iv := rg.WorkoutSteps[0]
	if iv.StepOrder != 3 || iv.StepType.StepTypeKey != "interval" {
		t.Errorf("interval: order %d type %s", iv.StepOrder, iv.StepType.StepTypeKey)
	}
	if iv.TargetType.WorkoutTargetTypeKey != "pace.zone" {
		t.Fatalf("interval target: %s", iv.TargetType.WorkoutTargetTypeKey)
	}
	if !approx(*iv.TargetValueOne, 1000.0/290) || !approx(*iv.TargetValueTwo, 1000.0/280) {
		t.Errorf("pace zone m/s: one=%f two=%f", *iv.TargetValueOne, *iv.TargetValueTwo)
	}

	rc := rg.WorkoutSteps[1]
	if rc.StepOrder != 4 || rc.EndCondition.ConditionTypeKey != "time" || *rc.EndConditionValue != 90 {
		t.Errorf("recovery: %+v end %+v", rc.StepOrder, rc.EndCondition)
	}

	cd := top[2]
	if cd.StepOrder != 5 || cd.StepType.StepTypeKey != "cooldown" {
		t.Errorf("cooldown: order %d type %s", cd.StepOrder, cd.StepType.StepTypeKey)
	}
}

func TestBuildWorkoutOpenStep(t *testing.T) {
	w, err := BuildWorkout("Easy run", []Step{{Kind: "run"}})
	if err != nil {
		t.Fatal(err)
	}
	s := w.WorkoutSegments[0].WorkoutSteps[0]
	if s.EndCondition.ConditionTypeKey != "lap.button" || s.EndConditionValue != nil {
		t.Errorf("open step end: %+v", s.EndCondition)
	}
}

func TestBuildWorkoutEmpty(t *testing.T) {
	if _, err := BuildWorkout("x", nil); err == nil {
		t.Fatal("want error for empty steps")
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
