package metrics

import (
	"sort"
	"time"
)

type Options struct {
	AsOf             time.Time
	Overrides        ThresholdOverrides
	RecentActivities int
	WeeklyHistory    int
}

func (o Options) withDefaults() Options {
	if o.RecentActivities == 0 {
		o.RecentActivities = 10
	}
	if o.WeeklyHistory == 0 {
		o.WeeklyHistory = 12
	}
	return o
}

type Snapshot struct {
	AsOf        time.Time
	Thresholds  Thresholds
	Fitness     FitnessPoint
	HasFitness  bool
	Trajectory  TrajectoryAssessment
	Recovery    RecoveryStatus
	Performance PerformanceModel
	Weekly      []WeekSummary
	Recent      []ActivityMetrics
	ActivityN   int
}

func BuildSnapshot(acts []Activity, wellness []Wellness, opts Options) Snapshot {
	opts = opts.withDefaults()
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = latestStart(acts)
	}

	thr := EstimateThresholds(acts, opts.Overrides)
	daily := DailyLoads(acts, thr)
	series := ComputeFitness(daily, asOf)

	snap := Snapshot{
		AsOf:        asOf,
		Thresholds:  thr,
		Trajectory:  ClassifyTrajectory(series),
		Recovery:    AssessRecovery(series, wellness, asOf),
		Performance: BuildPerformanceModel(acts, thr),
		Weekly:      tailWeeks(WeeklySummaries(acts, thr), opts.WeeklyHistory),
		Recent:      recentAnalyses(acts, thr, opts.RecentActivities),
		ActivityN:   len(acts),
	}
	if p, ok := series.Latest(); ok {
		snap.Fitness = p
		snap.HasFitness = true
	}
	return snap
}

func latestStart(acts []Activity) time.Time {
	var t time.Time
	for _, a := range acts {
		if a.Start.After(t) {
			t = a.Start
		}
	}
	return t
}

func recentAnalyses(acts []Activity, thr Thresholds, n int) []ActivityMetrics {
	sorted := append([]Activity(nil), acts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.After(sorted[j].Start) })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	out := make([]ActivityMetrics, 0, len(sorted))
	for _, a := range sorted {
		out = append(out, Analyze(a, thr))
	}
	return out
}

func tailWeeks(ws []WeekSummary, n int) []WeekSummary {
	if len(ws) <= n {
		return ws
	}
	return ws[len(ws)-n:]
}
