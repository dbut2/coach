package metrics

import "math"

// gradeAdjustFactor is the metabolic cost of running at gradient g (rise/run)
// relative to the flat, from Minetti et al. (2002) energy-cost polynomial.
func gradeAdjustFactor(g float64) float64 {
	g = clamp(g, -0.45, 0.45)
	cr := 155.4*math.Pow(g, 5) - 30.4*math.Pow(g, 4) - 43.3*math.Pow(g, 3) + 46.3*g*g + 19.5*g + 3.6
	const flat = 3.6
	return math.Max(0.2, cr/flat)
}

func gradeAdjustedSpeeds(s *Stream) []float64 {
	n := s.Len()
	if n < 2 || len(s.PaceSPerKm) != n {
		return nil
	}
	out := make([]float64, n)
	hasAlt := len(s.AltitudeM) == n
	for i := 0; i < n; i++ {
		v := speedFromPace(s.PaceSPerKm[i])
		if hasAlt && i > 0 {
			dt := float64(s.TimeOffsetS[i] - s.TimeOffsetS[i-1])
			dist := v * dt
			if dist > 0.5 {
				grade := (s.AltitudeM[i] - s.AltitudeM[i-1]) / dist
				v *= gradeAdjustFactor(grade)
			}
		}
		out[i] = v
	}
	return out
}

// normalized applies the 30-second rolling average then 4th-power mean used for
// normalized power and graded pace (Coggan), which weights surges supra-linearly.
func normalized(times []int, vals []float64) float64 {
	n := len(times)
	if n == 0 || len(vals) != n {
		return 0
	}
	smoothed := rollingMean(times, vals, 30)
	var sum float64
	var cnt int
	for _, v := range smoothed {
		if v > 0 {
			sum += math.Pow(v, 4)
			cnt++
		}
	}
	if cnt == 0 {
		return 0
	}
	return math.Pow(sum/float64(cnt), 0.25)
}

func rollingMean(times []int, vals []float64, window int) []float64 {
	n := len(times)
	out := make([]float64, n)
	var sum float64
	j := 0
	for i := 0; i < n; i++ {
		sum += vals[i]
		for times[i]-times[j] > window && j < i {
			sum -= vals[j]
			j++
		}
		out[i] = sum / float64(i-j+1)
	}
	return out
}

func intsToFloats(xs []int) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = float64(x)
	}
	return out
}

func normalizedGradedSpeed(s *Stream) float64 {
	speeds := gradeAdjustedSpeeds(s)
	if speeds == nil {
		return 0
	}
	return normalized(s.TimeOffsetS, speeds)
}

func normalizedPower(s *Stream) float64 {
	if !s.HasPower() || len(s.TimeOffsetS) != len(s.PowerW) {
		return 0
	}
	return normalized(s.TimeOffsetS, intsToFloats(s.PowerW))
}
