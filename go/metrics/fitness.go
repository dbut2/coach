package metrics

import (
	"math"
	"sort"
	"time"
)

type DailyLoad struct {
	Date     time.Time
	Load     float64
	RunKm    float64
	Duration int
}

type FitnessPoint struct {
	Date     time.Time
	Load     float64
	CTL      float64
	ATL      float64
	TSB      float64
	ACWR     float64
	Ramp     float64
	Monotony float64
	Strain   float64
}

type FitnessSeries []FitnessPoint

const (
	ctlDays = 42
	atlDays = 7
)

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func DailyLoads(acts []Activity, t Thresholds) []DailyLoad {
	byDay := map[time.Time]*DailyLoad{}
	for _, a := range acts {
		d := dayOf(a.Start)
		dl := byDay[d]
		if dl == nil {
			dl = &DailyLoad{Date: d}
			byDay[d] = dl
		}
		dl.Load += ComputeLoad(a, t).Chosen.Value
		dl.Duration += durationSeconds(a)
		if a.Sport.IsRun() {
			dl.RunKm += a.DistanceM / 1000
		}
	}
	out := make([]DailyLoad, 0, len(byDay))
	for _, dl := range byDay {
		dl.Load = round1(dl.Load)
		dl.RunKm = round2(dl.RunKm)
		out = append(out, *dl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// ComputeFitness builds the daily fitness series. Loads are filled to a
// continuous day grid (rest days = 0) so the exponentially-weighted CTL/ATL
// decay correctly; asOf extends the grid to "today" so recent rest shows up.
func ComputeFitness(daily []DailyLoad, asOf time.Time) FitnessSeries {
	if len(daily) == 0 {
		return nil
	}
	loadByDay := map[time.Time]DailyLoad{}
	for _, d := range daily {
		loadByDay[dayOf(d.Date)] = d
	}

	start := dayOf(daily[0].Date)
	end := dayOf(daily[len(daily)-1].Date)
	if !asOf.IsZero() && dayOf(asOf).After(end) {
		end = dayOf(asOf)
	}

	ctlA := 1 - math.Exp(-1.0/ctlDays)
	atlA := 1 - math.Exp(-1.0/atlDays)

	var series FitnessSeries
	var ctl, atl float64
	var window []float64
	var acute7, chronic28 []float64

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		load := loadByDay[d].Load
		ctl = ctl + atlSafe(ctlA)*(load-ctl)
		atl = atl + atlSafe(atlA)*(load-atl)

		window = appendCap(window, load, 7)
		acute7 = appendCap(acute7, load, 7)
		chronic28 = appendCap(chronic28, load, 28)

		p := FitnessPoint{
			Date: d,
			Load: load,
			CTL:  round1(ctl),
			ATL:  round1(atl),
			TSB:  round1(ctl - atl),
		}
		p.ACWR = round2(acwr(acute7, chronic28))
		p.Monotony, p.Strain = monotonyStrain(window)
		series = append(series, p)
	}

	for i := range series {
		if i >= 7 {
			series[i].Ramp = round1(series[i].CTL - series[i-7].CTL)
		}
	}
	return series
}

func atlSafe(a float64) float64 { return clamp(a, 0, 1) }

func acwr(acute7, chronic28 []float64) float64 {
	if len(chronic28) == 0 {
		return 0
	}
	a := mean(acute7)
	c := mean(chronic28)
	if c <= 0 {
		return 0
	}
	return a / c
}

func monotonyStrain(window []float64) (float64, float64) {
	if len(window) < 2 {
		return 0, 0
	}
	m := mean(window)
	sd := stddev(window)
	if sd <= 0 {
		return 0, 0
	}
	mono := m / sd
	var sum float64
	for _, v := range window {
		sum += v
	}
	return round2(mono), round1(sum * mono)
}

func appendCap(xs []float64, v float64, limit int) []float64 {
	xs = append(xs, v)
	if len(xs) > limit {
		xs = xs[len(xs)-limit:]
	}
	return xs
}

func (s FitnessSeries) Latest() (FitnessPoint, bool) {
	if len(s) == 0 {
		return FitnessPoint{}, false
	}
	return s[len(s)-1], true
}
