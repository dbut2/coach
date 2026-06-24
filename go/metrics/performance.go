package metrics

import (
	"math"
	"time"
)

type BestEffort struct {
	DistanceM float64
	DurationS int
	SpeedMS   float64
}

type RacePrediction struct {
	Label      string
	DistanceM  float64
	Time       time.Duration
	PaceSPerKm float64
	Method     string
}

type PerformanceModel struct {
	VO2Max          float64
	VDOT            float64
	CriticalSpeedMS float64
	DPrimeM         float64
	BestEfforts     []BestEffort
	Predictions     []RacePrediction
}

var standardDurations = []int{60, 120, 300, 600, 1200, 1800, 2700, 3600}

var raceDistances = []struct {
	Label string
	M     float64
}{
	{"1k", 1000},
	{"5k", 5000},
	{"10k", 10000},
	{"half_marathon", 21097.5},
	{"marathon", 42195},
}

func BuildPerformanceModel(acts []Activity, thr Thresholds) PerformanceModel {
	pm := PerformanceModel{CriticalSpeedMS: thr.CriticalSpeedMS}
	pm.BestEfforts = bestEfforts(acts)

	cs, dprime := criticalSpeedTwoParam(pm.BestEfforts)
	if cs > 0 {
		pm.CriticalSpeedMS = round2(cs)
		pm.DPrimeM = round1(dprime)
	}

	if best, ok := bestEffortNear(pm.BestEfforts, 1800, 3600); ok {
		pm.VO2Max = round1(vo2maxFromEffort(best.DistanceM, best.DurationS))
		pm.VDOT = pm.VO2Max
	}

	pm.Predictions = predictRaces(pm.BestEfforts, cs, dprime)
	return pm
}

func bestEfforts(acts []Activity) []BestEffort {
	best := make([]float64, len(standardDurations))
	for _, a := range acts {
		if !a.Sport.IsRun() {
			continue
		}
		dist := cumulativeDistance(a)
		if dist == nil {
			continue
		}
		for i, d := range standardDurations {
			if m := bestRollingDistance(a.Stream.TimeOffsetS, dist, d); m > best[i] {
				best[i] = m
			}
		}
	}
	var out []BestEffort
	for i, d := range standardDurations {
		if best[i] > 0 {
			out = append(out, BestEffort{
				DistanceM: round1(best[i]),
				DurationS: d,
				SpeedMS:   round2(best[i] / float64(d)),
			})
		}
	}
	return out
}

func bestEffortNear(efforts []BestEffort, lo, hi int) (BestEffort, bool) {
	var chosen BestEffort
	found := false
	for _, e := range efforts {
		if e.DurationS >= lo && e.DurationS <= hi {
			if !found || e.SpeedMS > chosen.SpeedMS {
				chosen = e
				found = true
			}
		}
	}
	if found {
		return chosen, true
	}
	for _, e := range efforts {
		if !found || e.DurationS > chosen.DurationS {
			chosen = e
			found = true
		}
	}
	return chosen, found
}

// vo2maxFromEffort applies Daniels & Gilbert's VO2 cost/percent-max model to a
// single distance/time effort, yielding VDOT (an effective VO2max).
func vo2maxFromEffort(distM float64, timeS int) float64 {
	if distM <= 0 || timeS <= 0 {
		return 0
	}
	t := float64(timeS) / 60
	v := distM / t
	vo2 := -4.60 + 0.182258*v + 0.000104*v*v
	pct := 0.8 + 0.1894393*math.Exp(-0.012778*t) + 0.2989558*math.Exp(-0.1932605*t)
	if pct <= 0 {
		return 0
	}
	return vo2 / pct
}

// criticalSpeedTwoParam fits distance = CS·t + D' over best efforts; CS is the
// asymptotic sustainable speed and D' the finite supra-CS distance capacity.
func criticalSpeedTwoParam(efforts []BestEffort) (float64, float64) {
	var xs, ys []float64
	for _, e := range efforts {
		if e.DurationS >= 120 && e.DurationS <= 1800 {
			xs = append(xs, float64(e.DurationS))
			ys = append(ys, e.DistanceM)
		}
	}
	if len(xs) < 2 {
		return 0, 0
	}
	cs, dprime := linregress(xs, ys)
	if cs <= 0 {
		return 0, 0
	}
	return cs, dprime
}

// predictRaces uses Riegel's endurance law T2 = T1·(D2/D1)^1.06 from the best
// effort, cross-checked against the critical-speed model where it applies.
func predictRaces(efforts []BestEffort, cs, dprime float64) []RacePrediction {
	anchor, ok := longestEffort(efforts)
	if !ok {
		return nil
	}
	var preds []RacePrediction
	for _, rd := range raceDistances {
		var secs float64
		method := "riegel"
		secs = float64(anchor.DurationS) * math.Pow(rd.M/anchor.DistanceM, 1.06)
		if cs > 0 && dprime > 0 && rd.M > dprime {
			csSecs := (rd.M - dprime) / cs
			if csSecs > 0 {
				secs = (secs + csSecs) / 2
				method = "riegel+cs"
			}
		}
		if secs <= 0 {
			continue
		}
		preds = append(preds, RacePrediction{
			Label:      rd.Label,
			DistanceM:  rd.M,
			Time:       time.Duration(secs) * time.Second,
			PaceSPerKm: round1(secs / (rd.M / 1000)),
			Method:     method,
		})
	}
	return preds
}

func longestEffort(efforts []BestEffort) (BestEffort, bool) {
	var chosen BestEffort
	found := false
	for _, e := range efforts {
		if !found || e.DistanceM > chosen.DistanceM {
			chosen = e
			found = true
		}
	}
	return chosen, found
}
