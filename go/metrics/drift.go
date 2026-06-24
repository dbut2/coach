package metrics

type Drift struct {
	FirstHalfHR  float64
	SecondHalfHR float64
	DriftPct     float64
	HRMin        int
	HRMax        int
	HRAvg        float64
	Decoupling   float64
	Reading      string
	HasData      bool
}

// CardiacDrift compares first- vs second-half average HR and, when output is
// available, the HR-to-output efficiency between halves (aerobic decoupling).
func CardiacDrift(a Activity) Drift {
	s := a.Stream
	if !s.HasHR() {
		return Drift{}
	}
	n := len(s.HR)
	mid := n / 2
	if mid == 0 {
		return Drift{}
	}

	first := meanPositiveInt(s.HR[:mid])
	second := meanPositiveInt(s.HR[mid:])
	d := Drift{
		FirstHalfHR:  round1(first),
		SecondHalfHR: round1(second),
		HRAvg:        round1(meanPositiveInt(s.HR)),
		HasData:      true,
	}
	d.HRMin, d.HRMax = minMaxPositive(s.HR)
	if first > 0 {
		d.DriftPct = round1(100 * (second - first) / first)
	}
	d.Decoupling = round1(aerobicDecoupling(a, mid))
	d.Reading = driftReading(d.DriftPct)
	return d
}

func driftReading(pct float64) string {
	switch {
	case pct > 3.5:
		return "faded or pushed harder late"
	case pct < -3.5:
		return "warmed in, settled — controlled"
	default:
		return "steady effort"
	}
}

// aerobicDecoupling is the percent rise in HR-per-output from first to second
// half; >5% conventionally marks the aerobic system decoupling under fatigue.
func aerobicDecoupling(a Activity, mid int) float64 {
	s := a.Stream
	var out []float64
	switch {
	case s.HasPower() && len(s.PowerW) == len(s.HR):
		out = intsToFloats(s.PowerW)
	case len(s.PaceSPerKm) == len(s.HR):
		out = make([]float64, len(s.PaceSPerKm))
		for i, p := range s.PaceSPerKm {
			out[i] = speedFromPace(p)
		}
	default:
		return 0
	}

	r1 := ratioOutputPerHR(out[:mid], s.HR[:mid])
	r2 := ratioOutputPerHR(out[mid:], s.HR[mid:])
	if r1 <= 0 {
		return 0
	}
	return 100 * (r1 - r2) / r1
}

func ratioOutputPerHR(out []float64, hr []int) float64 {
	var so, sh float64
	var n int
	for i := range out {
		if out[i] > 0 && hr[i] > 0 {
			so += out[i]
			sh += float64(hr[i])
			n++
		}
	}
	if n == 0 || sh == 0 {
		return 0
	}
	return (so / float64(n)) / (sh / float64(n))
}

func meanPositiveInt(xs []int) float64 {
	var sum float64
	var n int
	for _, x := range xs {
		if x > 0 {
			sum += float64(x)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func minMaxPositive(xs []int) (int, int) {
	mn, mx := 0, 0
	for _, x := range xs {
		if x <= 0 {
			continue
		}
		if mn == 0 || x < mn {
			mn = x
		}
		if x > mx {
			mx = x
		}
	}
	return mn, mx
}
