package source

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"naomi.run/database"
	"naomi.run/metrics"
)

type Loader struct {
	q database.Querier
}

func New(q database.Querier) *Loader {
	return &Loader{q: q}
}

type Data struct {
	Activities []metrics.Activity
	Wellness   []metrics.Wellness
}

func (l *Loader) Load(ctx context.Context, userID uuid.UUID, since time.Time) (Data, error) {
	acts, err := l.q.ListActivitiesByUser(ctx, database.ListActivitiesByUserParams{UserID: userID, StartTime: since})
	if err != nil {
		return Data{}, err
	}
	streamRows, err := l.q.ListActivityStreamsByUser(ctx, database.ListActivityStreamsByUserParams{UserID: userID, StartTime: since})
	if err != nil {
		return Data{}, err
	}
	wells, err := l.q.ListWellnessByUser(ctx, database.ListWellnessByUserParams{UserID: userID, Date: since})
	if err != nil {
		return Data{}, err
	}

	streams := map[uuid.UUID]*metrics.Stream{}
	for _, s := range streamRows {
		streams[s.ActivityID] = mapStream(s)
	}

	out := Data{Wellness: pivotWellness(wells)}
	for _, a := range acts {
		out.Activities = append(out.Activities, mapActivity(a, streams[a.ID]))
	}
	return out, nil
}

func (l *Loader) Snapshot(ctx context.Context, userID uuid.UUID, since time.Time, opts metrics.Options) (metrics.Snapshot, error) {
	d, err := l.Load(ctx, userID, since)
	if err != nil {
		return metrics.Snapshot{}, err
	}
	return metrics.BuildSnapshot(d.Activities, d.Wellness, opts), nil
}

// HR fields are absent here because the upstream Strava payload does not carry them.
type rawSummary struct {
	Distance       float64    `json:"distance"`
	MovingTime     int        `json:"moving_time"`
	ElapsedTime    int        `json:"elapsed_time"`
	TotalElevation float64    `json:"total_elevation_gain"`
	AverageSpeed   float64    `json:"average_speed"`
	MaxSpeed       float64    `json:"max_speed"`
	AverageWatts   float64    `json:"average_watts"`
	WeightedWatts  float64    `json:"weighted_average_watts"`
	MaxWatts       float64    `json:"max_watts"`
	AverageCadence float64    `json:"average_cadence"`
	Calories       float64    `json:"calories"`
	Trainer        bool       `json:"trainer"`
	Manual         bool       `json:"manual"`
	SplitsMetric   []rawSplit `json:"splits_metric"`
}

type rawSplit struct {
	Distance            float64 `json:"distance"`
	MovingTime          int     `json:"moving_time"`
	ElapsedTime         int     `json:"elapsed_time"`
	AverageSpeed        float64 `json:"average_speed"`
	ElevationDifference float64 `json:"elevation_difference"`
}

func mapActivity(a database.Activity, stream *metrics.Stream) metrics.Activity {
	var rs rawSummary
	_ = json.Unmarshal(a.RawSummary, &rs)

	act := metrics.Activity{
		ID:             a.ID.String(),
		Start:          a.StartTime,
		Sport:          metrics.ClassifySport(a.SportType),
		DistanceM:      rs.Distance,
		MovingTimeS:    rs.MovingTime,
		ElapsedTimeS:   rs.ElapsedTime,
		ElevationGainM: rs.TotalElevation,
		AvgSpeedMS:     rs.AverageSpeed,
		MaxSpeedMS:     rs.MaxSpeed,
		AvgPowerW:      rs.AverageWatts,
		WeightedPowerW: rs.WeightedWatts,
		MaxPowerW:      rs.MaxWatts,
		AvgCadence:     rs.AverageCadence,
		Calories:       rs.Calories,
		Trainer:        rs.Trainer,
		Manual:         rs.Manual,
		Stream:         stream,
	}
	for _, s := range rs.SplitsMetric {
		act.Splits = append(act.Splits, metrics.Split{
			DistanceM:      s.Distance,
			MovingTimeS:    s.MovingTime,
			ElapsedTimeS:   s.ElapsedTime,
			AvgSpeedMS:     s.AverageSpeed,
			ElevationDiffM: s.ElevationDifference,
		})
	}
	return act
}

func mapStream(s database.ActivityStream) *metrics.Stream {
	return &metrics.Stream{
		TimeOffsetS: int32sToInts(s.TimeOffsetS),
		HR:          int32sToInts(s.Hr),
		PaceSPerKm:  numericToFloats(s.PaceSPerKm),
		Cadence:     int32sToInts(s.Cadence),
		PowerW:      int32sToInts(s.PowerW),
		AltitudeM:   numericToFloats(s.AltitudeM),
		Lat:         numericToFloats(s.Lat),
		Lng:         numericToFloats(s.Lng),
	}
}

const (
	wellnessHRV         = "hrv"
	wellnessRestingHR   = "resting_hr"
	wellnessSleepMin    = "sleep_minutes"
	wellnessStress      = "stress_level"
	wellnessBodyBattery = "body_battery"
	wellnessReadiness   = "readiness"
)

func pivotWellness(rows []database.WellnessMetric) []metrics.Wellness {
	byDate := map[time.Time]*metrics.Wellness{}
	var order []time.Time
	for _, r := range rows {
		d := r.Date
		w := byDate[d]
		if w == nil {
			w = &metrics.Wellness{Date: d}
			byDate[d] = w
			order = append(order, d)
		}
		v := nullNumeric(r.ValueNum)
		switch r.MetricKey {
		case wellnessHRV:
			w.HRV = v
		case wellnessRestingHR:
			w.RestingHR = v
		case wellnessSleepMin:
			w.SleepMin = v
		case wellnessStress:
			w.Stress = v
		case wellnessBodyBattery:
			w.BodyBattery = v
		case wellnessReadiness:
			w.Readiness = v
		}
	}
	out := make([]metrics.Wellness, 0, len(order))
	for _, d := range order {
		out = append(out, *byDate[d])
	}
	return out
}

func int32sToInts(xs []int32) []int {
	if xs == nil {
		return nil
	}
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}
	return out
}

func numericToFloats(xs []string) []float64 {
	if xs == nil {
		return nil
	}
	out := make([]float64, len(xs))
	for i, s := range xs {
		f, _ := strconv.ParseFloat(s, 64)
		out[i] = f
	}
	return out
}

func nullNumeric(ns sql.NullString) float64 {
	if !ns.Valid {
		return 0
	}
	f, _ := strconv.ParseFloat(ns.String, 64)
	return f
}
