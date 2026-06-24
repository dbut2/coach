package metrics

import "time"

type ActivityMetrics struct {
	ActivityID    string
	Start         time.Time
	Sport         SportType
	DistanceKm    float64
	DurationS     int
	ElevationGain float64

	Load          Load
	IntensityZone string

	AvgPaceSPerKm float64
	GAPSPerKm     float64
	AvgSpeedKmh   float64
	AvgPowerW     float64
	NormPowerW    float64

	HRZones   ZoneDistribution
	PaceZones ZoneDistribution
	Drift     Drift

	NegativeSplit bool
}

func Analyze(a Activity, t Thresholds) ActivityMetrics {
	m := ActivityMetrics{
		ActivityID:    a.ID,
		Start:         a.Start,
		Sport:         a.Sport,
		DistanceKm:    round2(a.DistanceM / 1000),
		DurationS:     durationSeconds(a),
		ElevationGain: round1(a.ElevationGainM),
		Load:          ComputeLoad(a, t),
		AvgPowerW:     round1(a.AvgPowerW),
		NormPowerW:    round1(normalizedPower(a.Stream)),
		HRZones:       TimeInHRZones(a.Stream, t),
		PaceZones:     TimeInPaceZones(a.Stream, t),
		Drift:         CardiacDrift(a),
		NegativeSplit: negativeSplit(a),
	}
	m.IntensityZone = intensityZone(m.Load.IntensityFactor)

	if a.MovingTimeS > 0 && a.DistanceM > 0 {
		v := a.DistanceM / float64(a.MovingTimeS)
		m.AvgSpeedKmh = round2(v * 3.6)
		if a.Sport.IsRun() {
			m.AvgPaceSPerKm = round1(1000 / v)
		}
	}
	if ngs := normalizedGradedSpeed(a.Stream); ngs > 0 {
		m.GAPSPerKm = round1(1000 / ngs)
	}
	return m
}

func negativeSplit(a Activity) bool {
	if len(a.Splits) >= 2 {
		mid := len(a.Splits) / 2
		return paceOfSplits(a.Splits[mid:]) < paceOfSplits(a.Splits[:mid])
	}
	d := CardiacDrift(a)
	return d.HasData && d.DriftPct < -1 && a.Sport.IsRun()
}

func paceOfSplits(ss []Split) float64 {
	var dist float64
	var t int
	for _, s := range ss {
		dist += s.DistanceM
		t += s.MovingTimeS
	}
	if dist <= 0 {
		return 0
	}
	return float64(t) / (dist / 1000)
}
