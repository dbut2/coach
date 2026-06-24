package metrics

import (
	"math"
	"sort"
	"strings"
	"time"
)

type SportType string

const (
	SportRunning  SportType = "running"
	SportCycling  SportType = "cycling"
	SportSwimming SportType = "swimming"
	SportWalking  SportType = "walking"
	SportHiking   SportType = "hiking"
	SportRowing   SportType = "rowing"
	SportOther    SportType = "other"
)

func (s SportType) IsRun() bool { return s == SportRunning }

func (s SportType) Endurance() bool {
	switch s {
	case SportRunning, SportCycling, SportSwimming, SportWalking, SportHiking, SportRowing:
		return true
	default:
		return false
	}
}

func ClassifySport(raw string) SportType {
	r := strings.ToLower(raw)
	switch {
	case strings.Contains(r, "run"), strings.Contains(r, "treadmill"):
		return SportRunning
	case strings.Contains(r, "ride"), strings.Contains(r, "cycl"), strings.Contains(r, "bike"), strings.Contains(r, "handcycle"), strings.Contains(r, "velomobile"):
		return SportCycling
	case strings.Contains(r, "swim"):
		return SportSwimming
	case strings.Contains(r, "walk"):
		return SportWalking
	case strings.Contains(r, "hike"):
		return SportHiking
	case strings.Contains(r, "row"), strings.Contains(r, "kayak"), strings.Contains(r, "canoe"):
		return SportRowing
	default:
		return SportOther
	}
}

type Confidence string

const (
	ConfidenceNone   Confidence = "none"
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Activity carries no HR summary: the upstream payload omits it, so HR comes from Stream.
type Activity struct {
	ID             string
	Start          time.Time
	Sport          SportType
	DistanceM      float64
	MovingTimeS    int
	ElapsedTimeS   int
	ElevationGainM float64
	AvgSpeedMS     float64
	MaxSpeedMS     float64
	AvgPowerW      float64
	WeightedPowerW float64
	MaxPowerW      float64
	AvgCadence     float64
	Calories       float64
	Trainer        bool
	Manual         bool
	Splits         []Split
	Stream         *Stream
}

type Split struct {
	DistanceM      float64
	MovingTimeS    int
	ElapsedTimeS   int
	AvgSpeedMS     float64
	ElevationDiffM float64
}

// Stream slices are independently nil when the tracker did not record them.
type Stream struct {
	TimeOffsetS []int
	HR          []int
	PaceSPerKm  []float64
	Cadence     []int
	PowerW      []int
	AltitudeM   []float64
	Lat         []float64
	Lng         []float64
}

func (s *Stream) HasHR() bool       { return s != nil && len(s.HR) > 0 }
func (s *Stream) HasPower() bool    { return s != nil && len(s.PowerW) > 0 }
func (s *Stream) HasPace() bool     { return s != nil && len(s.PaceSPerKm) > 0 }
func (s *Stream) HasAltitude() bool { return s != nil && len(s.AltitudeM) > 0 }

func (s *Stream) Len() int {
	if s == nil {
		return 0
	}
	return len(s.TimeOffsetS)
}

func (s *Stream) Duration() int {
	if s == nil || len(s.TimeOffsetS) == 0 {
		return 0
	}
	return s.TimeOffsetS[len(s.TimeOffsetS)-1] - s.TimeOffsetS[0]
}

type Wellness struct {
	Date        time.Time
	HRV         float64
	RestingHR   float64
	SleepMin    float64
	Stress      float64
	BodyBattery float64
	Readiness   float64
}

func (w Wellness) HasHRV() bool       { return w.HRV > 0 }
func (w Wellness) HasRestingHR() bool { return w.RestingHR > 0 }
func (w Wellness) HasSleep() bool     { return w.SleepMin > 0 }

func durationSeconds(a Activity) int {
	if a.MovingTimeS > 0 {
		return a.MovingTimeS
	}
	if a.ElapsedTimeS > 0 {
		return a.ElapsedTimeS
	}
	return a.Stream.Duration()
}

func percentileInt(xs []int, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	if p <= 0 {
		return float64(cp[0])
	}
	if p >= 1 {
		return float64(cp[len(cp)-1])
	}
	pos := p * float64(len(cp)-1)
	lo := int(pos)
	frac := pos - float64(lo)
	if lo+1 >= len(cp) {
		return float64(cp[lo])
	}
	return float64(cp[lo])*(1-frac) + float64(cp[lo+1])*frac
}

func meanInts(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)-1))
}

func clamp(x, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, x))
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }
