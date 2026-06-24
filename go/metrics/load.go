package metrics

type LoadMethod string

const (
	MethodPowerTSS LoadMethod = "power_tss"
	MethodRunTSS   LoadMethod = "run_tss"
	MethodHRTSS    LoadMethod = "hr_tss"
	MethodTRIMP    LoadMethod = "trimp"
	MethodDuration LoadMethod = "duration"
)

type MethodLoad struct {
	Method     LoadMethod
	Value      float64
	Confidence Confidence
}

type Load struct {
	Chosen          MethodLoad
	ByMethod        map[LoadMethod]MethodLoad
	IntensityFactor float64
	DurationS       int

	chosenRank int
}

// ComputeLoad evaluates every load method the data supports and picks the most
// trustworthy: power > pace > HR > duration. TSS-family values are on the
// canonical "100 = one hour at threshold" scale so they are comparable.
func ComputeLoad(a Activity, t Thresholds) Load {
	l := Load{ByMethod: map[LoadMethod]MethodLoad{}, DurationS: durationSeconds(a)}

	if v, conf, ifac, ok := powerTSS(a, t); ok {
		l.ByMethod[MethodPowerTSS] = MethodLoad{MethodPowerTSS, v, conf}
		l.choose(MethodPowerTSS, ifac, 4)
	}
	if v, conf, ifac, ok := runTSS(a, t); ok {
		l.ByMethod[MethodRunTSS] = MethodLoad{MethodRunTSS, v, conf}
		l.choose(MethodRunTSS, ifac, 3)
	}
	if v, conf, ifac, ok := hrTSS(a, t); ok {
		l.ByMethod[MethodHRTSS] = MethodLoad{MethodHRTSS, v, conf}
		l.choose(MethodHRTSS, ifac, 2)
	}
	if v, conf, ok := trimp(a, t); ok {
		l.ByMethod[MethodTRIMP] = MethodLoad{MethodTRIMP, v, conf}
		l.choose(MethodTRIMP, 0, 1)
	}
	if l.Chosen.Method == "" {
		v := durationLoad(a)
		l.ByMethod[MethodDuration] = MethodLoad{MethodDuration, v, ConfidenceLow}
		l.Chosen = l.ByMethod[MethodDuration]
	}
	return l
}

func (l *Load) choose(m LoadMethod, ifac float64, rank int) {
	if l.chosenRank >= rank {
		return
	}
	l.chosenRank = rank
	l.Chosen = l.ByMethod[m]
	if ifac > 0 {
		l.IntensityFactor = round2(ifac)
	}
}

func tssFromIF(seconds int, ifac float64) float64 {
	hours := float64(seconds) / 3600
	return round1(hours * ifac * ifac * 100)
}

func powerTSS(a Activity, t Thresholds) (float64, Confidence, float64, bool) {
	if t.FTPWatts <= 0 {
		return 0, "", 0, false
	}
	np := normalizedPower(a.Stream)
	if np <= 0 {
		np = a.WeightedPowerW
	}
	if np <= 0 {
		np = a.AvgPowerW
	}
	if np <= 0 {
		return 0, "", 0, false
	}
	ifac := np / t.FTPWatts
	conf := ConfidenceHigh
	if !a.Stream.HasPower() {
		conf = ConfidenceMedium
	}
	return tssFromIF(durationSeconds(a), ifac), conf, ifac, true
}

func runTSS(a Activity, t Thresholds) (float64, Confidence, float64, bool) {
	if !a.Sport.IsRun() || t.CriticalSpeedMS <= 0 {
		return 0, "", 0, false
	}
	ngs := normalizedGradedSpeed(a.Stream)
	conf := ConfidenceHigh
	if ngs <= 0 {
		if a.MovingTimeS > 0 && a.DistanceM > 0 {
			ngs = a.DistanceM / float64(a.MovingTimeS)
			conf = ConfidenceMedium
		} else {
			return 0, "", 0, false
		}
	}
	ifac := ngs / t.CriticalSpeedMS
	return tssFromIF(durationSeconds(a), ifac), conf, ifac, true
}

func hrTSS(a Activity, t Thresholds) (float64, Confidence, float64, bool) {
	if !a.Stream.HasHR() || t.ThresholdHR <= 0 || t.MaxHR <= 0 {
		return 0, "", 0, false
	}
	rest := float64(t.RestingHR)
	thrReserve := float64(t.ThresholdHR) - rest
	if thrReserve <= 0 {
		return 0, "", 0, false
	}
	var sum float64
	var n int
	for _, hr := range a.Stream.HR {
		if hr > 0 {
			sum += (float64(hr) - rest) / thrReserve
			n++
		}
	}
	if n == 0 {
		return 0, "", 0, false
	}
	ifac := sum / float64(n)
	return tssFromIF(durationSeconds(a), ifac), ConfidenceMedium, ifac, true
}

// trimp is Edwards' summated HR-zone load: minutes in HR zone i weighted by i,
// avoiding the sex coefficient of the Banister exponential form.
func trimp(a Activity, t Thresholds) (float64, Confidence, bool) {
	dist := TimeInHRZones(a.Stream, t)
	if dist.Total == 0 {
		return 0, "", false
	}
	var score float64
	for _, b := range dist.Buckets {
		score += float64(b.Seconds) / 60 * float64(b.Index)
	}
	return round1(score), ConfidenceMedium, true
}

func durationLoad(a Activity) float64 {
	hours := float64(durationSeconds(a)) / 3600
	factor := 50.0
	switch a.Sport {
	case SportRunning:
		factor = 70
	case SportCycling, SportRowing:
		factor = 55
	case SportSwimming:
		factor = 65
	case SportWalking, SportHiking:
		factor = 40
	}
	return round1(hours * factor)
}

func intensityZone(ifac float64) string {
	switch {
	case ifac <= 0:
		return "unknown"
	case ifac < 0.75:
		return "recovery"
	case ifac < 0.85:
		return "endurance"
	case ifac < 0.95:
		return "tempo"
	case ifac < 1.05:
		return "threshold"
	default:
		return "vo2max"
	}
}
