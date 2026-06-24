package strava

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	api "naomi.run/clients/strava"
	"naomi.run/clients/strava/activities"
	"naomi.run/clients/strava/models"
	"naomi.run/clients/strava/streams"
)

type Client struct {
	api *api.StravaAPIV3
}

func NewClient(httpClient *http.Client) *Client {
	rt := httptransport.NewWithClient(api.DefaultHost, api.DefaultBasePath, api.DefaultSchemes, httpClient)
	return &Client{api: api.New(rt, strfmt.Default)}
}

type Activity struct {
	ID        int64
	StartTime time.Time
	SportType string
	Raw       json.RawMessage
}

type Streams struct {
	Time     []int32
	HR       []int32
	Pace     []float64
	Cadence  []int32
	Power    []int32
	Altitude []float64
	Lat      []float64
	Lng      []float64
}

func (c *Client) Activity(ctx context.Context, id int64) (*Activity, error) {
	p := activities.NewGetActivityByIDParams()
	p.ID = id

	resp, err := c.api.Activities.GetActivityByIDContext(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	d := resp.Payload
	a, err := toActivity(d.ID, d.StartDate, string(d.SportType), d)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (c *Client) Activities(ctx context.Context, after, before time.Time) ([]Activity, error) {
	const perPage = int64(100)
	a, b, pp := after.Unix(), before.Unix(), perPage

	var out []Activity
	for page := int64(1); ; page++ {
		pg := page
		p := activities.NewGetLoggedInAthleteActivitiesParams()
		p.After, p.Before, p.Page, p.PerPage = &a, &b, &pg, &pp

		resp, err := c.api.Activities.GetLoggedInAthleteActivitiesContext(ctx, p, nil)
		if err != nil {
			return nil, err
		}
		for _, sa := range resp.Payload {
			act, err := toActivity(sa.ID, sa.StartDate, string(sa.SportType), sa)
			if err != nil {
				return nil, err
			}
			out = append(out, act)
		}
		if int64(len(resp.Payload)) < perPage {
			return out, nil
		}
	}
}

func toActivity(id int64, start strfmt.DateTime, sport string, payload any) (Activity, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Activity{}, err
	}
	return Activity{ID: id, StartTime: time.Time(start), SportType: strings.ToLower(sport), Raw: raw}, nil
}

var streamKeys = []string{"time", "heartrate", "cadence", "watts", "altitude", "latlng", "velocity_smooth"}

// Streams returns nil when the activity has no stream data (manual entries, treadmill, etc).
func (c *Client) Streams(ctx context.Context, id int64) (*Streams, error) {
	p := streams.NewGetActivityStreamsParams()
	p.ID = id
	p.Keys = streamKeys
	p.KeyByType = true

	resp, err := c.api.Streams.GetActivityStreamsContext(ctx, p, nil)
	if err != nil {
		if d, ok := errors.AsType[*streams.GetActivityStreamsDefault](err); ok && d.Code() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return mapStreams(resp.Payload), nil
}

func mapStreams(ss *models.StreamSet) *Streams {
	if ss == nil {
		return nil
	}
	s := &Streams{}
	if ss.Time != nil {
		s.Time = int32s(ss.Time.Data)
	}
	if ss.Heartrate != nil {
		s.HR = int32s(ss.Heartrate.Data)
	}
	if ss.Cadence != nil {
		s.Cadence = int32s(ss.Cadence.Data)
	}
	if ss.Watts != nil {
		s.Power = int32s(ss.Watts.Data)
	}
	if ss.Altitude != nil {
		s.Altitude = float64sf(ss.Altitude.Data)
	}
	if ss.VelocitySmooth != nil {
		s.Pace = paceFromVelocity(ss.VelocitySmooth.Data)
	}
	if ss.Latlng != nil {
		s.Lat = make([]float64, len(ss.Latlng.Data))
		s.Lng = make([]float64, len(ss.Latlng.Data))
		for i, ll := range ss.Latlng.Data {
			if len(ll) >= 2 {
				s.Lat[i], s.Lng[i] = float64(ll[0]), float64(ll[1])
			}
		}
	}
	return s
}

func paceFromVelocity(v []float32) []float64 {
	pace := make([]float64, len(v))
	for i, mps := range v {
		if mps > 0 {
			pace[i] = 1000 / float64(mps)
		}
	}
	return pace
}

func int32s(xs []int64) []int32 {
	out := make([]int32, len(xs))
	for i, x := range xs {
		out[i] = int32(x)
	}
	return out
}

func float64sf(xs []float32) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = float64(x)
	}
	return out
}
