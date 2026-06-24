package coach

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"naomi.run/metrics"
)

type MetricsSource interface {
	Snapshot(ctx context.Context, userID uuid.UUID, since time.Time, opts metrics.Options) (metrics.Snapshot, error)
}

const snapshotLookback = 78 * 7 * 24 * time.Hour

func (c *Coach) snapshot(tc agent.ToolContext, opts metrics.Options) (metrics.Snapshot, error) {
	id, err := uuid.Parse(tc.UserID())
	if err != nil {
		return metrics.Snapshot{}, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
	}
	return c.src.Snapshot(tc, id, time.Now().Add(-snapshotLookback), opts)
}

func (c *Coach) metricsTools() ([]tool.Tool, error) {
	status, err := functiontool.New(functiontool.Config{
		Name:        "training_status",
		Description: "Current training state for the athlete: fitness (CTL), fatigue (ATL), form (TSB), acute:chronic workload ratio, ramp and trajectory, recovery readiness (with HRV and sleep when available), estimated thresholds, and race-time predictions. Use for big-picture questions about how training is going and whether to push or back off.",
	}, func(tc agent.ToolContext, _ struct{}) (trainingStatus, error) {
		snap, err := c.snapshot(tc, metrics.Options{})
		if err != nil {
			return trainingStatus{}, err
		}
		return toTrainingStatus(snap), nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	weekly, err := functiontool.New(functiontool.Config{
		Name:        "weekly_training",
		Description: "Week-by-week training history, oldest first: run distance, time, count, elevation, average pace, training load, and cross-training. Use for mileage and volume trends. These totals are precomputed for you — quote them, do not re-add.",
	}, func(tc agent.ToolContext, in weeklyInput) ([]weekDTO, error) {
		weeks := in.Weeks
		if weeks <= 0 {
			weeks = 12
		}
		snap, err := c.snapshot(tc, metrics.Options{WeeklyHistory: weeks})
		if err != nil {
			return nil, err
		}
		return toWeekly(snap.Weekly), nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	recent, err := functiontool.New(functiontool.Config{
		Name:        "recent_activities",
		Description: "Detailed per-activity analysis for recent sessions, most recent first: pace, grade-adjusted pace, intensity zone, load, average and max HR, HR drift, aerobic decoupling, time-in-zone distribution, and negative-split flag. Use to see how individual runs actually went. Defaults to the last 8 weeks.",
	}, func(tc agent.ToolContext, in recentInput) ([]activityDTO, error) {
		opts := metrics.Options{}
		if in.Days > 0 {
			opts.RecentWindow = time.Duration(in.Days) * 24 * time.Hour
		}
		snap, err := c.snapshot(tc, opts)
		if err != nil {
			return nil, err
		}
		acts := snap.Recent
		if in.Limit > 0 && len(acts) > in.Limit {
			acts = acts[:in.Limit]
		}
		return toActivities(acts), nil
	})
	if err != nil {
		return nil, fmt.Errorf("coach: init tools: %w", err)
	}

	return []tool.Tool{status, weekly, recent}, nil
}

type weeklyInput struct {
	Weeks int `json:"weeks,omitempty" jsonschema:"how many recent weeks to return; defaults to 12"`
}

type recentInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of activities to return, most recent first; 0 means all in range"`
	Days  int `json:"days,omitempty" jsonschema:"how many days back to include; defaults to 56 (8 weeks)"`
}

type trainingStatus struct {
	AsOf        string         `json:"as_of,omitempty"`
	Activities  int            `json:"activity_count"`
	Fitness     *fitnessDTO    `json:"fitness,omitempty"`
	Trajectory  trajectoryDTO  `json:"trajectory"`
	Recovery    recoveryDTO    `json:"recovery"`
	Thresholds  thresholdsDTO  `json:"thresholds"`
	Performance performanceDTO `json:"performance"`
}

type fitnessDTO struct {
	Date     string  `json:"date,omitempty"`
	CTL      float64 `json:"ctl" jsonschema:"chronic training load (fitness)"`
	ATL      float64 `json:"atl" jsonschema:"acute training load (fatigue)"`
	TSB      float64 `json:"tsb" jsonschema:"training stress balance (form = ctl - atl)"`
	ACWR     float64 `json:"acwr" jsonschema:"acute:chronic workload ratio; above 1.5 is injury-risk territory"`
	Ramp     float64 `json:"ramp_per_week" jsonschema:"weekly change in ctl"`
	Monotony float64 `json:"monotony"`
	Strain   float64 `json:"strain"`
}

type trajectoryDTO struct {
	State string   `json:"state" jsonschema:"building, maintaining, detraining, spiking, tapering, or detrained"`
	Note  string   `json:"note,omitempty"`
	Flags []string `json:"flags,omitempty"`
}

type recoveryDTO struct {
	Score           float64  `json:"score" jsonschema:"recovery readiness 0-100"`
	State           string   `json:"state"`
	Recommendation  string   `json:"recommendation,omitempty"`
	TSB             float64  `json:"tsb"`
	ACWR            float64  `json:"acwr"`
	Monotony        float64  `json:"monotony"`
	HRVStatus       string   `json:"hrv_status,omitempty"`
	HRVDeviationPct float64  `json:"hrv_deviation_pct,omitempty"`
	SleepDebtMin    float64  `json:"sleep_debt_min,omitempty"`
	SuggestedRest   int      `json:"suggested_rest_days"`
	Inputs          []string `json:"inputs,omitempty" jsonschema:"which signals fed the score (training_load, hrv, sleep)"`
}

type thresholdsDTO struct {
	MaxHR         int               `json:"max_hr,omitempty"`
	RestingHR     int               `json:"resting_hr,omitempty"`
	ThresholdHR   int               `json:"threshold_hr,omitempty"`
	ThresholdPace string            `json:"threshold_pace,omitempty"`
	FTPWatts      float64           `json:"ftp_watts,omitempty"`
	Confidence    map[string]string `json:"confidence,omitempty" jsonschema:"per-threshold estimate confidence (none/low/medium/high)"`
}

type performanceDTO struct {
	VO2Max        float64       `json:"vo2max,omitempty"`
	VDOT          float64       `json:"vdot,omitempty"`
	CriticalSpeed float64       `json:"critical_speed_ms,omitempty"`
	Predictions   []racePredDTO `json:"race_predictions,omitempty"`
}

type racePredDTO struct {
	Race   string `json:"race"`
	Time   string `json:"time"`
	Pace   string `json:"pace"`
	Method string `json:"method,omitempty"`
}

type weekDTO struct {
	WeekStart      string  `json:"week_start"`
	RunDistanceKm  float64 `json:"run_distance_km"`
	RunTime        string  `json:"run_time,omitempty"`
	Runs           int     `json:"runs"`
	ElevationM     float64 `json:"run_elevation_m,omitempty"`
	AvgPace        string  `json:"avg_run_pace,omitempty"`
	Load           float64 `json:"load"`
	CrossTrainTime string  `json:"cross_train_time,omitempty"`
	CrossTrains    int     `json:"cross_trains,omitempty"`
}

type activityDTO struct {
	ID            string    `json:"id"`
	Date          string    `json:"date"`
	Sport         string    `json:"sport"`
	DistanceKm    float64   `json:"distance_km"`
	Duration      string    `json:"duration,omitempty"`
	ElevationGain float64   `json:"elevation_gain_m,omitempty"`
	AvgPace       string    `json:"avg_pace,omitempty"`
	AvgSpeedKmh   float64   `json:"avg_speed_kmh,omitempty"`
	GAP           string    `json:"grade_adjusted_pace,omitempty"`
	Load          float64   `json:"load"`
	Intensity     string    `json:"intensity_zone,omitempty"`
	AvgHR         float64   `json:"avg_hr,omitempty"`
	MaxHR         int       `json:"max_hr,omitempty"`
	HRDriftPct    float64   `json:"hr_drift_pct,omitempty" jsonschema:"second-half vs first-half HR change; positive means cardiac drift"`
	Decoupling    float64   `json:"aerobic_decoupling_pct,omitempty"`
	NegativeSplit bool      `json:"negative_split"`
	HRZones       []zoneDTO `json:"hr_zones,omitempty"`
	PaceZones     []zoneDTO `json:"pace_zones,omitempty"`
}

type zoneDTO struct {
	Zone    string  `json:"zone"`
	Percent float64 `json:"percent"`
}

func toTrainingStatus(s metrics.Snapshot) trainingStatus {
	ts := trainingStatus{
		AsOf:        fmtDate(s.AsOf),
		Activities:  s.ActivityN,
		Trajectory:  trajectoryDTO{State: string(s.Trajectory.Trajectory), Note: s.Trajectory.Note, Flags: s.Trajectory.Flags},
		Recovery:    toRecovery(s.Recovery),
		Thresholds:  toThresholds(s.Thresholds),
		Performance: toPerformance(s.Performance),
	}
	if s.HasFitness {
		f := s.Fitness
		ts.Fitness = &fitnessDTO{
			Date:     fmtDate(f.Date),
			CTL:      f.CTL,
			ATL:      f.ATL,
			TSB:      f.TSB,
			ACWR:     f.ACWR,
			Ramp:     f.Ramp,
			Monotony: f.Monotony,
			Strain:   f.Strain,
		}
	}
	return ts
}

func toRecovery(r metrics.RecoveryStatus) recoveryDTO {
	return recoveryDTO{
		Score:           r.Score,
		State:           r.State,
		Recommendation:  r.Recommendation,
		TSB:             r.TSB,
		ACWR:            r.ACWR,
		Monotony:        r.Monotony,
		HRVStatus:       r.HRVStatus,
		HRVDeviationPct: r.HRVDeviation,
		SleepDebtMin:    r.SleepDebtMin,
		SuggestedRest:   r.SuggestedRest,
		Inputs:          r.Inputs,
	}
}

func toThresholds(t metrics.Thresholds) thresholdsDTO {
	dto := thresholdsDTO{
		MaxHR:         t.MaxHR,
		RestingHR:     t.RestingHR,
		ThresholdHR:   t.ThresholdHR,
		ThresholdPace: fmtPace(t.ThresholdPaceSPerKm),
		FTPWatts:      t.FTPWatts,
	}
	if len(t.Sources) > 0 {
		dto.Confidence = make(map[string]string, len(t.Sources))
		for k, src := range t.Sources {
			dto.Confidence[k] = string(src.Confidence)
		}
	}
	return dto
}

func toPerformance(p metrics.PerformanceModel) performanceDTO {
	dto := performanceDTO{VO2Max: p.VO2Max, VDOT: p.VDOT, CriticalSpeed: p.CriticalSpeedMS}
	for _, pr := range p.Predictions {
		dto.Predictions = append(dto.Predictions, racePredDTO{
			Race:   pr.Label,
			Time:   fmtDurD(pr.Time),
			Pace:   fmtPace(pr.PaceSPerKm),
			Method: pr.Method,
		})
	}
	return dto
}

func toWeekly(ws []metrics.WeekSummary) []weekDTO {
	out := make([]weekDTO, 0, len(ws))
	for _, w := range ws {
		d := weekDTO{
			WeekStart:     fmtDate(w.WeekStart),
			RunDistanceKm: w.RunDistanceKm,
			RunTime:       fmtDur(w.RunTimeS),
			Runs:          w.RunCount,
			ElevationM:    w.RunElevationM,
			AvgPace:       fmtPace(w.AvgRunPaceSPerKm),
			Load:          w.Load,
		}
		if w.CrossTrainCount > 0 {
			d.CrossTrainTime = fmtDur(w.CrossTrainTimeS)
			d.CrossTrains = w.CrossTrainCount
		}
		out = append(out, d)
	}
	return out
}

func toActivities(ms []metrics.ActivityMetrics) []activityDTO {
	out := make([]activityDTO, 0, len(ms))
	for _, m := range ms {
		d := activityDTO{
			ID:            m.ActivityID,
			Date:          fmtDate(m.Start),
			Sport:         string(m.Sport),
			DistanceKm:    m.DistanceKm,
			Duration:      fmtDur(m.DurationS),
			ElevationGain: m.ElevationGain,
			Load:          m.Load.Chosen.Value,
			Intensity:     m.IntensityZone,
			NegativeSplit: m.NegativeSplit,
			HRZones:       toZones(m.HRZones),
			PaceZones:     toZones(m.PaceZones),
		}
		if m.AvgPaceSPerKm > 0 {
			d.AvgPace = fmtPace(m.AvgPaceSPerKm)
		} else {
			d.AvgSpeedKmh = m.AvgSpeedKmh
		}
		if m.GAPSPerKm > 0 {
			d.GAP = fmtPace(m.GAPSPerKm)
		}
		if m.Drift.HasData {
			d.AvgHR = m.Drift.HRAvg
			d.MaxHR = m.Drift.HRMax
			d.HRDriftPct = m.Drift.DriftPct
			d.Decoupling = m.Drift.Decoupling
		}
		out = append(out, d)
	}
	return out
}

func toZones(z metrics.ZoneDistribution) []zoneDTO {
	if z.Total == 0 {
		return nil
	}
	var out []zoneDTO
	for _, b := range z.Buckets {
		if b.Percent <= 0 {
			continue
		}
		out = append(out, zoneDTO{Zone: b.Name, Percent: b.Percent})
	}
	return out
}

func fmtPace(sPerKm float64) string {
	if sPerKm <= 0 {
		return ""
	}
	s := int(math.Round(sPerKm))
	return fmt.Sprintf("%d:%02d/km", s/60, s%60)
}

func fmtDur(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	h := seconds / 3600
	m := seconds % 3600 / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func fmtDurD(d time.Duration) string { return fmtDur(int(d.Seconds())) }

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
