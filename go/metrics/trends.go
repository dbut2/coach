package metrics

import (
	"sort"
	"time"
)

type WeekSummary struct {
	WeekStart        time.Time
	RunDistanceKm    float64
	RunTimeS         int
	RunCount         int
	RunElevationM    float64
	CrossTrainTimeS  int
	CrossTrainCount  int
	Load             float64
	AvgRunPaceSPerKm float64
}

func weekStart(t time.Time) time.Time {
	d := dayOf(t)
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

func WeeklySummaries(acts []Activity, thr Thresholds) []WeekSummary {
	byWeek := map[time.Time]*WeekSummary{}
	for _, a := range acts {
		w := weekStart(a.Start)
		ws := byWeek[w]
		if ws == nil {
			ws = &WeekSummary{WeekStart: w}
			byWeek[w] = ws
		}
		ws.Load += ComputeLoad(a, thr).Chosen.Value
		if a.Sport.IsRun() {
			ws.RunCount++
			ws.RunDistanceKm += a.DistanceM / 1000
			ws.RunTimeS += a.MovingTimeS
			ws.RunElevationM += a.ElevationGainM
		} else if a.Sport.Endurance() {
			ws.CrossTrainCount++
			ws.CrossTrainTimeS += durationSeconds(a)
		}
	}

	out := make([]WeekSummary, 0, len(byWeek))
	for _, ws := range byWeek {
		if ws.RunDistanceKm > 0 && ws.RunTimeS > 0 {
			ws.AvgRunPaceSPerKm = round1(float64(ws.RunTimeS) / ws.RunDistanceKm)
		}
		ws.RunDistanceKm = round2(ws.RunDistanceKm)
		ws.RunElevationM = round1(ws.RunElevationM)
		ws.Load = round1(ws.Load)
		out = append(out, *ws)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WeekStart.Before(out[j].WeekStart) })
	return out
}

type Trajectory string

const (
	TrajectoryBuilding    Trajectory = "building"
	TrajectoryMaintaining Trajectory = "maintaining"
	TrajectoryDetraining  Trajectory = "detraining"
	TrajectorySpiking     Trajectory = "spiking"
	TrajectoryTapering    Trajectory = "tapering"
	TrajectoryDetrained   Trajectory = "detrained"
)

type TrajectoryAssessment struct {
	Trajectory Trajectory
	Ramp       float64
	ACWR       float64
	TSB        float64
	Monotony   float64
	Flags      []string
	Note       string
}

// ClassifyTrajectory reads the recent fitness series for the athlete's
// direction. ACWR>1.5 or weekly ramp>8 CTL flags spiking injury risk; sustained
// negative ramp with low load flags detraining; high TSB after a CTL drop with
// low ACWR reads as a deliberate taper.
func ClassifyTrajectory(s FitnessSeries) TrajectoryAssessment {
	last, ok := s.Latest()
	if !ok {
		return TrajectoryAssessment{Trajectory: TrajectoryMaintaining, Note: "no data"}
	}
	a := TrajectoryAssessment{
		Ramp:     last.Ramp,
		ACWR:     last.ACWR,
		TSB:      last.TSB,
		Monotony: last.Monotony,
	}

	if last.ACWR > 1.5 && last.ACWR > 0 {
		a.Flags = append(a.Flags, "acwr above 1.5 (injury-risk zone)")
	}
	if last.Ramp > 8 {
		a.Flags = append(a.Flags, "ctl ramp above 8/week")
	}
	if last.Monotony > 2.0 {
		a.Flags = append(a.Flags, "high training monotony")
	}

	recentLoad := recentMeanLoad(s, 14)
	switch {
	case last.CTL < 15 && recentLoad < 10:
		a.Trajectory = TrajectoryDetrained
	case (last.ACWR > 1.5 || last.Ramp > 8) && recentLoad > 0:
		a.Trajectory = TrajectorySpiking
	case last.Ramp < -3 && last.TSB > 10 && last.ACWR > 0 && last.ACWR < 0.9:
		a.Trajectory = TrajectoryTapering
	case last.Ramp < -2:
		a.Trajectory = TrajectoryDetraining
	case last.Ramp > 2:
		a.Trajectory = TrajectoryBuilding
	default:
		a.Trajectory = TrajectoryMaintaining
	}
	a.Note = trajectoryNote(a.Trajectory)
	return a
}

func recentMeanLoad(s FitnessSeries, days int) float64 {
	if len(s) == 0 {
		return 0
	}
	from := len(s) - days
	if from < 0 {
		from = 0
	}
	var loads []float64
	for _, p := range s[from:] {
		loads = append(loads, p.Load)
	}
	return mean(loads)
}

func trajectoryNote(t Trajectory) string {
	switch t {
	case TrajectorySpiking:
		return "load is climbing faster than the body is adapting"
	case TrajectoryBuilding:
		return "fitness trending up at a sustainable rate"
	case TrajectoryMaintaining:
		return "holding fitness steady"
	case TrajectoryTapering:
		return "load easing while fitness is banked — taper shape"
	case TrajectoryDetraining:
		return "fitness slipping; load below maintenance"
	case TrajectoryDetrained:
		return "little recent training; base is low"
	default:
		return ""
	}
}
