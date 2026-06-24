package metrics

type Zone struct {
	Index int
	Name  string
	Lo    float64
	Hi    float64
}

type ZoneBucket struct {
	Zone
	Seconds int
	Percent float64
}

type ZoneDistribution struct {
	Basis   string
	Buckets []ZoneBucket
	Total   int
}

var hrZoneNames = []string{"recovery", "endurance", "tempo", "threshold", "vo2max"}

// HRZones returns 5 zones as fractions of HR reserve (Karvonen): Z1 <60%, Z2
// 60-70, Z3 70-80, Z4 80-90, Z5 >90%.
func HRZones(t Thresholds) []Zone {
	if t.MaxHR <= 0 {
		return nil
	}
	rest := float64(t.RestingHR)
	reserve := float64(t.MaxHR) - rest
	if reserve <= 0 {
		return nil
	}
	edges := []float64{0, 0.60, 0.70, 0.80, 0.90, 1.0}
	zones := make([]Zone, 5)
	for i := 0; i < 5; i++ {
		zones[i] = Zone{
			Index: i + 1,
			Name:  hrZoneNames[i],
			Lo:    rest + edges[i]*reserve,
			Hi:    rest + edges[i+1]*reserve,
		}
	}
	zones[4].Hi = float64(t.MaxHR)
	return zones
}

var paceZoneNames = []string{"easy", "marathon", "threshold", "interval", "repetition"}

// PaceZones returns running pace bands (s/km) as multiples of threshold pace,
// loosely after Daniels: easy >1.25×, marathon 1.10-1.25, threshold 0.97-1.10,
// interval 0.90-0.97, repetition <0.90 (faster = smaller s/km).
func PaceZones(t Thresholds) []Zone {
	tp := t.ThresholdPaceSPerKm
	if tp <= 0 {
		return nil
	}
	mult := []float64{2.5, 1.25, 1.10, 0.97, 0.90, 0.5}
	zones := make([]Zone, 5)
	for i := 0; i < 5; i++ {
		zones[i] = Zone{
			Index: i + 1,
			Name:  paceZoneNames[i],
			Lo:    tp * mult[i+1],
			Hi:    tp * mult[i],
		}
	}
	return zones
}

func TimeInHRZones(s *Stream, t Thresholds) ZoneDistribution {
	zones := HRZones(t)
	if zones == nil || !s.HasHR() {
		return ZoneDistribution{Basis: "hr"}
	}
	return distributeByValue(s.TimeOffsetS, intsToFloats(s.HR), zones, "hr")
}

func TimeInPaceZones(s *Stream, t Thresholds) ZoneDistribution {
	zones := PaceZones(t)
	if zones == nil || len(s.PaceSPerKm) != s.Len() || s.Len() == 0 {
		return ZoneDistribution{Basis: "pace"}
	}
	return distributeByValue(s.TimeOffsetS, s.PaceSPerKm, zones, "pace")
}

func distributeByValue(times []int, vals []float64, zones []Zone, basis string) ZoneDistribution {
	dist := ZoneDistribution{Basis: basis}
	secs := make([]int, len(zones))
	total := 0
	for i := 1; i < len(times); i++ {
		dt := times[i] - times[i-1]
		if dt <= 0 || dt > 60 {
			continue
		}
		v := vals[i]
		if v <= 0 {
			continue
		}
		zi := zoneIndexFor(v, zones)
		if zi >= 0 {
			secs[zi] += dt
			total += dt
		}
	}
	for i, z := range zones {
		pct := 0.0
		if total > 0 {
			pct = round1(100 * float64(secs[i]) / float64(total))
		}
		dist.Buckets = append(dist.Buckets, ZoneBucket{Zone: z, Seconds: secs[i], Percent: pct})
	}
	dist.Total = total
	return dist
}

func zoneIndexFor(v float64, zones []Zone) int {
	for i, z := range zones {
		if v >= z.Lo && v < z.Hi {
			return i
		}
	}
	if v >= zones[len(zones)-1].Hi {
		return len(zones) - 1
	}
	if v < zones[0].Lo {
		return 0
	}
	return -1
}
