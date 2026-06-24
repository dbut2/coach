package metrics

import "math"

type ThresholdSource struct {
	Method     string
	Confidence Confidence
	Samples    int
}

type Thresholds struct {
	MaxHR       int
	RestingHR   int
	ThresholdHR int

	ThresholdPaceSPerKm float64
	CriticalSpeedMS     float64

	FTPWatts float64

	Sources map[string]ThresholdSource
}

type ThresholdOverrides struct {
	MaxHR               int
	RestingHR           int
	ThresholdHR         int
	ThresholdPaceSPerKm float64
	FTPWatts            float64
}

func EstimateThresholds(acts []Activity, ov ThresholdOverrides) Thresholds {
	t := Thresholds{Sources: map[string]ThresholdSource{}}

	estimateHRThresholds(acts, &t)
	estimateRunThresholds(acts, &t)
	estimatePowerThreshold(acts, &t)

	applyThresholdOverrides(&t, ov)
	return t
}

func estimateHRThresholds(acts []Activity, t *Thresholds) {
	var all []int
	for _, a := range acts {
		if a.Stream.HasHR() {
			for _, hr := range a.Stream.HR {
				if hr > 0 {
					all = append(all, hr)
				}
			}
		}
	}
	if len(all) == 0 {
		return
	}

	t.MaxHR = int(math.Round(percentileInt(all, 0.999)))
	t.Sources["max_hr"] = ThresholdSource{Method: "observed p99.9", Confidence: hrConfidence(len(all)), Samples: len(all)}

	t.RestingHR = int(math.Round(percentileInt(all, 0.02)))
	t.Sources["resting_hr"] = ThresholdSource{Method: "observed p2 (in-activity floor)", Confidence: ConfidenceLow, Samples: len(all)}

	lthr, n := bestSustainedHR(acts, 30*60)
	if lthr > 0 {
		t.ThresholdHR = int(math.Round(lthr))
		t.Sources["threshold_hr"] = ThresholdSource{Method: "best 30-min mean HR", Confidence: hrConfidence(n), Samples: n}
	} else if t.MaxHR > 0 {
		t.ThresholdHR = int(math.Round(0.88 * float64(t.MaxHR)))
		t.Sources["threshold_hr"] = ThresholdSource{Method: "0.88 × max HR fallback", Confidence: ConfidenceLow}
	}
}

func bestSustainedHR(acts []Activity, window int) (float64, int) {
	best, count := 0.0, 0
	for _, a := range acts {
		s := a.Stream
		if !s.HasHR() || len(s.TimeOffsetS) != len(s.HR) {
			continue
		}
		count++
		if v := bestRollingMeanInt(s.TimeOffsetS, s.HR, window); v > best {
			best = v
		}
	}
	return best, count
}

func estimateRunThresholds(acts []Activity, t *Thresholds) {
	cs, n := criticalSpeed(acts)
	if cs <= 0 {
		return
	}
	t.CriticalSpeedMS = round2(cs)
	t.ThresholdPaceSPerKm = round1(1000 / cs)
	conf := ConfidenceLow
	if n >= 3 {
		conf = ConfidenceMedium
	}
	if n >= 8 {
		conf = ConfidenceHigh
	}
	t.Sources["threshold_pace"] = ThresholdSource{Method: "critical-speed fit of best efforts", Confidence: conf, Samples: n}
}

// Fits d = CS·t + D' (critical-speed model); the slope CS is sustainable speed.
func criticalSpeed(acts []Activity) (float64, int) {
	durations := []int{180, 300, 600, 1200, 1800, 3600}
	bestDist := make([]float64, len(durations))
	runs := 0
	for _, a := range acts {
		if !a.Sport.IsRun() {
			continue
		}
		s := a.Stream
		if s == nil || len(s.TimeOffsetS) < 2 {
			continue
		}
		dist := cumulativeDistance(a)
		if dist == nil {
			continue
		}
		runs++
		for i, d := range durations {
			if m := bestRollingDistance(s.TimeOffsetS, dist, d); m > bestDist[i] {
				bestDist[i] = m
			}
		}
	}

	var xs, ys []float64
	for i, d := range durations {
		if bestDist[i] > 0 {
			xs = append(xs, float64(d))
			ys = append(ys, bestDist[i])
		}
	}
	if len(xs) < 2 {
		return 0, runs
	}
	slope, _ := linregress(xs, ys)
	if slope <= 0 {
		return 0, runs
	}
	return slope, runs
}

func estimatePowerThreshold(acts []Activity, t *Thresholds) {
	best20, n := 0.0, 0
	for _, a := range acts {
		if a.Sport != SportCycling {
			continue
		}
		s := a.Stream
		if !s.HasPower() || len(s.TimeOffsetS) != len(s.PowerW) {
			continue
		}
		n++
		if v := bestRollingMeanInt(s.TimeOffsetS, s.PowerW, 20*60); v > best20 {
			best20 = v
		}
	}
	if best20 <= 0 {
		return
	}
	t.FTPWatts = round1(0.95 * best20)
	t.Sources["ftp"] = ThresholdSource{Method: "0.95 × best 20-min power", Confidence: hrConfidence(n), Samples: n}
}

func hrConfidence(samples int) Confidence {
	switch {
	case samples == 0:
		return ConfidenceNone
	case samples < 600:
		return ConfidenceLow
	case samples < 5000:
		return ConfidenceMedium
	default:
		return ConfidenceHigh
	}
}

func applyThresholdOverrides(t *Thresholds, ov ThresholdOverrides) {
	if ov.MaxHR > 0 {
		t.MaxHR = ov.MaxHR
		t.Sources["max_hr"] = ThresholdSource{Method: "override", Confidence: ConfidenceHigh}
	}
	if ov.RestingHR > 0 {
		t.RestingHR = ov.RestingHR
		t.Sources["resting_hr"] = ThresholdSource{Method: "override", Confidence: ConfidenceHigh}
	}
	if ov.ThresholdHR > 0 {
		t.ThresholdHR = ov.ThresholdHR
		t.Sources["threshold_hr"] = ThresholdSource{Method: "override", Confidence: ConfidenceHigh}
	}
	if ov.ThresholdPaceSPerKm > 0 {
		t.ThresholdPaceSPerKm = ov.ThresholdPaceSPerKm
		t.CriticalSpeedMS = round2(1000 / ov.ThresholdPaceSPerKm)
		t.Sources["threshold_pace"] = ThresholdSource{Method: "override", Confidence: ConfidenceHigh}
	}
	if ov.FTPWatts > 0 {
		t.FTPWatts = ov.FTPWatts
		t.Sources["ftp"] = ThresholdSource{Method: "override", Confidence: ConfidenceHigh}
	}
}

func cumulativeDistance(a Activity) []float64 {
	s := a.Stream
	if s == nil || len(s.TimeOffsetS) < 2 {
		return nil
	}
	n := len(s.TimeOffsetS)

	if len(s.Lat) == n && len(s.Lng) == n {
		d := make([]float64, n)
		for i := 1; i < n; i++ {
			d[i] = d[i-1] + haversine(s.Lat[i-1], s.Lng[i-1], s.Lat[i], s.Lng[i])
		}
		if d[n-1] > 0 {
			return d
		}
	}

	if len(s.PaceSPerKm) == n {
		d := make([]float64, n)
		for i := 1; i < n; i++ {
			dt := float64(s.TimeOffsetS[i] - s.TimeOffsetS[i-1])
			v := speedFromPace(s.PaceSPerKm[i])
			d[i] = d[i-1] + v*dt
		}
		if d[n-1] > 0 {
			return d
		}
	}

	if a.DistanceM > 0 && s.TimeOffsetS[n-1] > s.TimeOffsetS[0] {
		total := s.TimeOffsetS[n-1] - s.TimeOffsetS[0]
		d := make([]float64, n)
		for i := range d {
			d[i] = a.DistanceM * float64(s.TimeOffsetS[i]-s.TimeOffsetS[0]) / float64(total)
		}
		return d
	}
	return nil
}

func speedFromPace(paceSPerKm float64) float64 {
	if paceSPerKm <= 0 {
		return 0
	}
	return 1000 / paceSPerKm
}

func bestRollingDistance(times []int, cumDist []float64, window int) float64 {
	n := len(times)
	if n < 2 || len(cumDist) != n {
		return 0
	}
	best := 0.0
	j := 0
	for i := 0; i < n; i++ {
		for times[i]-times[j] > window && j < i {
			j++
		}
		if times[i]-times[j] >= window-1 && times[i] > times[j] {
			covered := cumDist[i] - cumDist[j]
			scaled := covered * float64(window) / float64(times[i]-times[j])
			if scaled > best {
				best = scaled
			}
		}
	}
	return best
}

func bestRollingMeanInt(times []int, vals []int, window int) float64 {
	n := len(times)
	if n == 0 || len(vals) != n {
		return 0
	}
	if times[n-1]-times[0] < window {
		return meanInts(vals)
	}
	best := 0.0
	var sum int
	j := 0
	for i := 0; i < n; i++ {
		sum += vals[i]
		for times[i]-times[j] > window && j < i {
			sum -= vals[j]
			j++
		}
		if times[i]-times[j] >= window-1 {
			m := float64(sum) / float64(i-j+1)
			if m > best {
				best = m
			}
		}
	}
	return best
}

func linregress(xs, ys []float64) (slope, intercept float64) {
	n := float64(len(xs))
	if n < 2 {
		return 0, 0
	}
	var sx, sy, sxx, sxy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
	}
	denom := n*sxx - sx*sx
	if denom == 0 {
		return 0, 0
	}
	slope = (n*sxy - sx*sy) / denom
	intercept = (sy - slope*sx) / n
	return slope, intercept
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
