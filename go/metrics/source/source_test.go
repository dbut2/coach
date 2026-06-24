package source

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"naomi.run/database"
	"naomi.run/metrics"
)

func TestMapActivityParsesRawSummary(t *testing.T) {
	raw := map[string]any{
		"distance":             10000.0,
		"moving_time":          3000,
		"elapsed_time":         3100,
		"total_elevation_gain": 120.5,
		"average_speed":        3.33,
		"average_watts":        210.0,
		"average_cadence":      88.0,
		"trainer":              true,
		"splits_metric": []map[string]any{
			{"distance": 1000.0, "moving_time": 300, "average_speed": 3.33, "elevation_difference": 5.0},
			{"distance": 1000.0, "moving_time": 295, "average_speed": 3.39, "elevation_difference": -2.0},
		},
	}
	b, _ := json.Marshal(raw)
	row := database.Activity{
		ID:         uuid.New(),
		SportType:  "run",
		StartTime:  time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC),
		RawSummary: b,
	}

	a := mapActivity(row, nil)
	if a.Sport != metrics.SportRunning {
		t.Errorf("sport = %s", a.Sport)
	}
	if a.DistanceM != 10000 || a.MovingTimeS != 3000 || a.ElapsedTimeS != 3100 {
		t.Errorf("scalars wrong: %+v", a)
	}
	if a.ElevationGainM != 120.5 || a.AvgPowerW != 210 || a.AvgCadence != 88 {
		t.Errorf("scalars wrong: %+v", a)
	}
	if !a.Trainer {
		t.Error("trainer flag lost")
	}
	if len(a.Splits) != 2 || a.Splits[1].MovingTimeS != 295 {
		t.Errorf("splits wrong: %+v", a.Splits)
	}
}

func TestMapStreamConvertsNumericArrays(t *testing.T) {
	row := database.ActivityStream{
		ActivityID:  uuid.New(),
		TimeOffsetS: []int32{0, 1, 2},
		Hr:          []int32{120, 130, 140},
		PaceSPerKm:  []string{"300.5", "299.0", "0"},
		AltitudeM:   []string{"10.0", "10.5", "11.0"},
	}
	s := mapStream(row)
	if s.Len() != 3 {
		t.Fatalf("len = %d", s.Len())
	}
	if s.HR[2] != 140 {
		t.Errorf("hr = %v", s.HR)
	}
	if s.PaceSPerKm[0] != 300.5 || s.PaceSPerKm[2] != 0 {
		t.Errorf("pace = %v", s.PaceSPerKm)
	}
	if s.AltitudeM[1] != 10.5 {
		t.Errorf("alt = %v", s.AltitudeM)
	}
}

func TestPivotWellnessGroupsByDate(t *testing.T) {
	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	rows := []database.WellnessMetric{
		{Date: d, MetricKey: wellnessHRV, ValueNum: sql.NullString{String: "65.5", Valid: true}},
		{Date: d, MetricKey: wellnessRestingHR, ValueNum: sql.NullString{String: "48", Valid: true}},
		{Date: d, MetricKey: wellnessSleepMin, ValueNum: sql.NullString{String: "455", Valid: true}},
		{Date: d.AddDate(0, 0, 1), MetricKey: wellnessHRV, ValueNum: sql.NullString{String: "70", Valid: true}},
	}
	out := pivotWellness(rows)
	if len(out) != 2 {
		t.Fatalf("days = %d, want 2", len(out))
	}
	if out[0].HRV != 65.5 || out[0].RestingHR != 48 || out[0].SleepMin != 455 {
		t.Errorf("day0 = %+v", out[0])
	}
	if out[1].HRV != 70 {
		t.Errorf("day1 = %+v", out[1])
	}
}
