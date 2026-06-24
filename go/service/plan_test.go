package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProposedDayDecodesGateDiff(t *testing.T) {
	diff := []byte(`{"date":"2026-07-10","workout":"rest","detail":"easy spin","distance_km":18.5,"duration_min":95}`)
	var d proposedDay
	if err := json.Unmarshal(diff, &d); err != nil {
		t.Fatalf("decode gate diff: %v", err)
	}
	pd, err := d.toPlanDay()
	if err != nil {
		t.Fatalf("toPlanDay: %v", err)
	}
	if pd.WorkoutType != "rest" || pd.Description != "easy spin" {
		t.Fatalf("fields lost: %+v", pd)
	}
	if pd.DistanceM != 18500 || pd.DurationS != 95*60 {
		t.Fatalf("conversion wrong: %v / %v", pd.DistanceM, pd.DurationS)
	}
	if !pd.Date.Equal(time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date parse wrong: %v", pd.Date)
	}
}

func TestWeekday(t *testing.T) {
	if got := weekday("2026-07-10"); got != "Fri" {
		t.Fatalf("weekday = %q, want Fri", got)
	}
	if got := weekday("garbage"); got != "" {
		t.Fatalf("weekday(garbage) = %q, want empty", got)
	}
}
