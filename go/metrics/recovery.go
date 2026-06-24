package metrics

import (
	"sort"
	"time"
)

type RecoveryStatus struct {
	Score          float64
	State          string
	TSB            float64
	ACWR           float64
	Monotony       float64
	HRVStatus      string
	HRVDeviation   float64
	SleepDebtMin   float64
	SuggestedRest  int
	Inputs         []string
	Recommendation string
}

// AssessRecovery blends training-derived readiness (form, acute:chronic ratio,
// monotony) with wellness signals (HRV vs baseline, sleep) when present. It
// degrades to training-only when no wellness data exists.
func AssessRecovery(s FitnessSeries, wellness []Wellness, asOf time.Time) RecoveryStatus {
	rs := RecoveryStatus{Score: 50}
	last, ok := s.Latest()
	if ok {
		rs.TSB = last.TSB
		rs.ACWR = last.ACWR
		rs.Monotony = last.Monotony
		rs.Inputs = append(rs.Inputs, "training_load")
		rs.Score = trainingReadiness(last)
	}

	if dev, status, hasHRV := hrvSignal(wellness, asOf); hasHRV {
		rs.HRVStatus = status
		rs.HRVDeviation = round1(dev)
		rs.Inputs = append(rs.Inputs, "hrv")
		rs.Score += hrvAdjustment(dev)
	}
	if debt, has := sleepDebt(wellness, asOf); has {
		rs.SleepDebtMin = round1(debt)
		rs.Inputs = append(rs.Inputs, "sleep")
		rs.Score -= clamp(debt/15, 0, 15)
	}

	rs.Score = round1(clamp(rs.Score, 0, 100))
	rs.State, rs.SuggestedRest, rs.Recommendation = recoveryVerdict(rs)
	return rs
}

// trainingReadiness maps form (TSB) to a 0-100 base, then penalises a high
// acute:chronic ratio and high monotony, the two strongest overtraining signals.
func trainingReadiness(p FitnessPoint) float64 {
	score := 50 + clamp(p.TSB, -40, 40)
	if p.ACWR > 1.3 {
		score -= clamp((p.ACWR-1.3)*40, 0, 25)
	}
	if p.ACWR > 0 && p.ACWR < 0.8 {
		score += 5
	}
	if p.Monotony > 2.0 {
		score -= clamp((p.Monotony-2.0)*10, 0, 10)
	}
	return score
}

func hrvSignal(wellness []Wellness, asOf time.Time) (float64, string, bool) {
	var hrvs []float64
	var latest float64
	var latestDate time.Time
	for _, w := range wellness {
		if !w.HasHRV() {
			continue
		}
		hrvs = append(hrvs, w.HRV)
		if w.Date.After(latestDate) {
			latestDate = w.Date
			latest = w.HRV
		}
	}
	if len(hrvs) < 3 || latest <= 0 {
		return 0, "", false
	}
	baseline := mean(hrvs)
	sd := stddev(hrvs)
	if baseline <= 0 {
		return 0, "", false
	}
	devPct := 100 * (latest - baseline) / baseline
	status := "balanced"
	if sd > 0 {
		switch {
		case latest < baseline-sd:
			status = "suppressed"
		case latest > baseline+sd:
			status = "elevated"
		}
	}
	return devPct, status, true
}

func hrvAdjustment(devPct float64) float64 {
	return clamp(devPct*0.5, -20, 15)
}

func sleepDebt(wellness []Wellness, asOf time.Time) (float64, bool) {
	const target = 480.0
	var recent []Wellness
	cutoff := dayOf(asOf).AddDate(0, 0, -7)
	for _, w := range wellness {
		if w.HasSleep() && (asOf.IsZero() || !w.Date.Before(cutoff)) {
			recent = append(recent, w)
		}
	}
	if len(recent) == 0 {
		return 0, false
	}
	sort.Slice(recent, func(i, j int) bool { return recent[i].Date.After(recent[j].Date) })
	if len(recent) > 3 {
		recent = recent[:3]
	}
	var debt float64
	for _, w := range recent {
		debt += target - w.SleepMin
	}
	return debt, true
}

func recoveryVerdict(rs RecoveryStatus) (string, int, string) {
	switch {
	case rs.Score >= 75:
		return "primed", 0, "recovered and ready for quality work"
	case rs.Score >= 55:
		return "ready", 0, "fresh enough for planned training"
	case rs.Score >= 40:
		return "moderate", 0, "manageable fatigue; keep intensity in check"
	case rs.Score >= 25:
		return "fatigued", 1, "carrying real fatigue; favour easy or a down day"
	default:
		return "depleted", 2, "high strain; prioritise recovery"
	}
}
