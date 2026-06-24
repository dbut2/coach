package metrics

import (
	"math"
	"testing"
	"time"
)

func steadyStream(durationS int, paceSPerKm float64, hr, power int, alt float64) *Stream {
	n := durationS + 1
	s := &Stream{}
	for i := 0; i < n; i++ {
		s.TimeOffsetS = append(s.TimeOffsetS, i)
		if paceSPerKm > 0 {
			s.PaceSPerKm = append(s.PaceSPerKm, paceSPerKm)
		}
		if hr > 0 {
			s.HR = append(s.HR, hr)
		}
		if power > 0 {
			s.PowerW = append(s.PowerW, power)
		}
		s.AltitudeM = append(s.AltitudeM, alt)
	}
	return s
}

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.3f, want %.3f (±%.3f)", name, got, want, tol)
	}
}

func TestClassifySport(t *testing.T) {
	cases := map[string]SportType{
		"Run": SportRunning, "trailrun": SportRunning, "treadmill": SportRunning,
		"Ride": SportCycling, "VirtualRide": SportCycling, "ebikeride": SportCycling,
		"swim": SportSwimming, "Hike": SportHiking, "walk": SportWalking,
		"kayaking": SportRowing, "WeightTraining": SportOther,
	}
	for in, want := range cases {
		if got := ClassifySport(in); got != want {
			t.Errorf("ClassifySport(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGradeAdjustFactorFlatIsOne(t *testing.T) {
	approx(t, "GAF(0)", gradeAdjustFactor(0), 1.0, 1e-9)
	if gradeAdjustFactor(0.1) <= 1.0 {
		t.Error("uphill should cost more than flat")
	}
	if gradeAdjustFactor(-0.05) >= 1.0 {
		t.Error("gentle downhill should cost less than flat")
	}
}

func TestTSSFromIF(t *testing.T) {
	approx(t, "1h @ IF 1.0", tssFromIF(3600, 1.0), 100, 1e-6)
	approx(t, "1h @ IF 0.5", tssFromIF(3600, 0.5), 25, 1e-6)
	approx(t, "30m @ IF 1.0", tssFromIF(1800, 1.0), 50, 1e-6)
}

func TestRunTSSAtThreshold(t *testing.T) {
	thr := Thresholds{CriticalSpeedMS: 4.0, ThresholdPaceSPerKm: 250}
	a := Activity{
		Sport:       SportRunning,
		MovingTimeS: 3600,
		DistanceM:   14400,
		Stream:      steadyStream(3600, 250, 0, 0, 0),
	}
	l := ComputeLoad(a, thr)
	if l.Chosen.Method != MethodRunTSS {
		t.Fatalf("chosen method = %s, want run_tss", l.Chosen.Method)
	}
	approx(t, "rTSS", l.Chosen.Value, 100, 1.0)
	approx(t, "IF", l.IntensityFactor, 1.0, 0.02)
}

func TestPowerTSSPrefersPower(t *testing.T) {
	thr := Thresholds{FTPWatts: 250, CriticalSpeedMS: 4, ThresholdHR: 160, MaxHR: 190, RestingHR: 50}
	a := Activity{
		Sport:       SportCycling,
		MovingTimeS: 3600,
		Stream:      steadyStream(3600, 0, 0, 250, 0),
	}
	l := ComputeLoad(a, thr)
	if l.Chosen.Method != MethodPowerTSS {
		t.Fatalf("chosen = %s, want power_tss", l.Chosen.Method)
	}
	approx(t, "pwrTSS", l.Chosen.Value, 100, 1.0)
}

func TestHRTSSAtThreshold(t *testing.T) {
	thr := Thresholds{ThresholdHR: 160, MaxHR: 190, RestingHR: 50}
	a := Activity{Sport: SportRunning, MovingTimeS: 3600, Stream: steadyStream(3600, 0, 160, 0, 0)}
	v, _, ifac, ok := hrTSS(a, thr)
	if !ok {
		t.Fatal("hrTSS not computed")
	}
	approx(t, "hrTSS IF", ifac, 1.0, 0.01)
	approx(t, "hrTSS", v, 100, 1.0)
}

func TestTrimpZoneWeighted(t *testing.T) {
	thr := Thresholds{MaxHR: 200, RestingHR: 50}
	a := Activity{Sport: SportRunning, MovingTimeS: 3600, Stream: steadyStream(3600, 0, 185, 0, 0)}
	v, _, ok := trimp(a, thr)
	if !ok {
		t.Fatal("trimp not computed")
	}
	approx(t, "trimp 60min in Z5", v, 300, 5)
}

func TestFitnessConstantLoadConverges(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var daily []DailyLoad
	for i := 0; i < 300; i++ {
		daily = append(daily, DailyLoad{Date: start.AddDate(0, 0, i), Load: 50})
	}
	series := ComputeFitness(daily, time.Time{})
	last, _ := series.Latest()
	approx(t, "CTL", last.CTL, 50, 0.2)
	approx(t, "ATL", last.ATL, 50, 0.2)
	approx(t, "TSB", last.TSB, 0, 0.2)
	approx(t, "ACWR", last.ACWR, 1.0, 0.05)
}

func TestFitnessRestExtendsToAsOf(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	daily := []DailyLoad{{Date: start, Load: 100}}
	asOf := start.AddDate(0, 0, 20)
	series := ComputeFitness(daily, asOf)
	if len(series) != 21 {
		t.Fatalf("series len = %d, want 21", len(series))
	}
	last, _ := series.Latest()
	if last.TSB <= 0 {
		t.Errorf("TSB after 20 rest days should be positive, got %.1f", last.TSB)
	}
}

func TestRiegelPrediction(t *testing.T) {
	efforts := []BestEffort{{DistanceM: 5000, DurationS: 1200, SpeedMS: 4.1667}}
	preds := predictRaces(efforts, 0, 0)
	var tenK *RacePrediction
	for i := range preds {
		if preds[i].Label == "10k" {
			tenK = &preds[i]
		}
	}
	if tenK == nil {
		t.Fatal("no 10k prediction")
	}
	want := 1200 * math.Pow(2, 1.06)
	approx(t, "10k seconds", tenK.Time.Seconds(), want, 1)
}

func TestWeekStartIsMonday(t *testing.T) {
	wed := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	ws := weekStart(wed)
	if ws.Weekday() != time.Monday {
		t.Errorf("weekStart weekday = %v, want Monday", ws.Weekday())
	}
	if ws.Day() != 22 {
		t.Errorf("weekStart day = %d, want 22", ws.Day())
	}
}

func TestTrajectorySpiking(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var daily []DailyLoad
	for i := 0; i < 28; i++ {
		daily = append(daily, DailyLoad{Date: start.AddDate(0, 0, i), Load: 20})
	}
	for i := 28; i < 42; i++ {
		daily = append(daily, DailyLoad{Date: start.AddDate(0, 0, i), Load: 130})
	}
	series := ComputeFitness(daily, time.Time{})
	tr := ClassifyTrajectory(series)
	if tr.Trajectory != TrajectorySpiking {
		t.Errorf("trajectory = %s, want spiking (ACWR=%.2f ramp=%.1f)", tr.Trajectory, tr.ACWR, tr.Ramp)
	}
}

func TestZoneDistributionSumsToTotal(t *testing.T) {
	thr := Thresholds{MaxHR: 200, RestingHR: 50}
	s := steadyStream(600, 0, 150, 0, 0)
	d := TimeInHRZones(s, thr)
	var sum int
	for _, b := range d.Buckets {
		sum += b.Seconds
	}
	if sum != d.Total {
		t.Errorf("zone seconds sum = %d, total = %d", sum, d.Total)
	}
	if d.Total == 0 {
		t.Error("expected non-zero dwell time")
	}
}

func TestCardiacDriftReading(t *testing.T) {
	s := &Stream{}
	for i := 0; i <= 600; i++ {
		s.TimeOffsetS = append(s.TimeOffsetS, i)
		hr := 140
		if i > 300 {
			hr = 160
		}
		s.HR = append(s.HR, hr)
	}
	d := CardiacDrift(Activity{Sport: SportRunning, Stream: s})
	if !d.HasData || d.DriftPct <= 0 {
		t.Fatalf("expected positive drift, got %.1f", d.DriftPct)
	}
	if d.Reading != "faded or pushed harder late" {
		t.Errorf("reading = %q", d.Reading)
	}
	if d.HRMax != 160 || d.HRMin != 140 {
		t.Errorf("HR range = %d-%d, want 140-160", d.HRMin, d.HRMax)
	}
}

func TestEstimateThresholdsFromHistory(t *testing.T) {
	start := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	var acts []Activity
	for i := 0; i < 5; i++ {
		acts = append(acts, Activity{
			ID:          string(rune('a' + i)),
			Start:       start.AddDate(0, 0, i*2),
			Sport:       SportRunning,
			MovingTimeS: 1800,
			DistanceM:   6000,
			Stream:      steadyStream(1800, 300, 165, 0, 0),
		})
	}
	thr := EstimateThresholds(acts, ThresholdOverrides{})
	if thr.MaxHR < 160 {
		t.Errorf("MaxHR = %d, want >=160", thr.MaxHR)
	}
	if thr.ThresholdPaceSPerKm <= 0 {
		t.Error("expected a threshold pace estimate")
	}
	if thr.Sources["max_hr"].Method == "" {
		t.Error("expected max_hr source metadata")
	}
}

func TestThresholdOverrideWins(t *testing.T) {
	thr := EstimateThresholds(nil, ThresholdOverrides{MaxHR: 195, ThresholdPaceSPerKm: 240})
	if thr.MaxHR != 195 {
		t.Errorf("MaxHR = %d, want 195", thr.MaxHR)
	}
	approx(t, "CS from override pace", thr.CriticalSpeedMS, 1000.0/240, 0.01)
}

func TestRecoveryDegradesWithoutWellness(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var daily []DailyLoad
	for i := 0; i < 60; i++ {
		daily = append(daily, DailyLoad{Date: start.AddDate(0, 0, i), Load: 50})
	}
	series := ComputeFitness(daily, time.Time{})
	rs := AssessRecovery(series, nil, time.Time{})
	if len(rs.Inputs) != 1 || rs.Inputs[0] != "training_load" {
		t.Errorf("inputs = %v, want [training_load]", rs.Inputs)
	}
	if rs.State == "" {
		t.Error("expected a recovery state")
	}
}

func TestBuildSnapshotEndToEnd(t *testing.T) {
	start := time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC)
	var acts []Activity
	for i := 0; i < 30; i++ {
		acts = append(acts, Activity{
			ID:          time.Duration(i).String(),
			Start:       start.AddDate(0, 0, i),
			Sport:       SportRunning,
			MovingTimeS: 2400,
			DistanceM:   8000,
			Stream:      steadyStream(2400, 300, 155, 0, 0),
		})
	}
	snap := BuildSnapshot(acts, nil, Options{})
	if snap.ActivityN != 30 {
		t.Errorf("ActivityN = %d", snap.ActivityN)
	}
	if !snap.HasFitness {
		t.Error("expected fitness series")
	}
	if len(snap.Recent) == 0 {
		t.Error("expected recent analyses")
	}
	if len(snap.Weekly) == 0 {
		t.Error("expected weekly summaries")
	}
	if snap.Thresholds.MaxHR == 0 {
		t.Error("expected derived thresholds")
	}
}
