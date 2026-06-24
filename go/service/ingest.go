package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"naomi.run/database"
	"naomi.run/strava"
)

const activityBackfillWindow = 18 * 7 * 24 * time.Hour

func (s *Service) stravaClient(ctx context.Context, conn database.StravaConnection) *strava.Client {
	tok := &oauth2.Token{
		AccessToken:  conn.AccessToken,
		RefreshToken: conn.RefreshToken,
		Expiry:       conn.ExpiresAt,
		TokenType:    "Bearer",
	}
	src := &savingTokenSource{
		ctx:    ctx,
		src:    s.oauth.TokenSource(ctx, tok),
		last:   conn.AccessToken,
		userID: conn.UserID,
		scope:  conn.Scope,
		q:      s.q,
	}
	return strava.NewClient(oauth2.NewClient(ctx, src))
}

type savingTokenSource struct {
	ctx    context.Context
	src    oauth2.TokenSource
	last   string
	userID uuid.UUID
	scope  string
	q      *database.Queries
}

func (t *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := t.src.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != t.last {
		t.last = tok.AccessToken
		if err := t.q.UpdateStravaTokens(t.ctx, database.UpdateStravaTokensParams{
			UserID:       t.userID,
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			ExpiresAt:    tok.Expiry,
			Scope:        t.scope,
		}); err != nil {
			return nil, err
		}
	}
	return tok, nil
}

func (s *Service) ingestActivity(ctx context.Context, conn database.StravaConnection, activityID int64) error {
	client := s.stravaClient(ctx, conn)

	act, err := client.Activity(ctx, activityID)
	if err != nil {
		return err
	}

	id, err := s.upsertActivity(ctx, conn.UserID, *act)
	if err != nil {
		return err
	}

	streams, err := client.Streams(ctx, activityID)
	if err != nil {
		return err
	}
	if streams == nil {
		return nil
	}
	return s.q.UpsertActivityStream(ctx, database.UpsertActivityStreamParams{
		ActivityID:  id,
		TimeOffsetS: streams.Time,
		Hr:          streams.HR,
		PaceSPerKm:  floats(streams.Pace),
		Cadence:     streams.Cadence,
		PowerW:      streams.Power,
		AltitudeM:   floats(streams.Altitude),
		Lat:         floats(streams.Lat),
		Lng:         floats(streams.Lng),
	})
}

func (s *Service) backfillActivities(ctx context.Context, conn database.StravaConnection, since time.Time) (int, error) {
	client := s.stravaClient(ctx, conn)

	acts, err := client.Activities(ctx, since, time.Now())
	if err != nil {
		return 0, err
	}

	var n int
	for _, a := range acts {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		if err := s.ingestActivity(ctx, conn, a.ID); err != nil {
			slog.Error("backfill: ingest", "activity_id", a.ID, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

func (s *Service) startBackfill(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	conn, err := s.q.GetStravaConnectionByUserID(ctx, userID)
	if err != nil {
		slog.Error("backfill: load connection", "user_id", userID, "err", err)
		return
	}
	n, err := s.backfillActivities(ctx, conn, time.Now().Add(-activityBackfillWindow))
	if err != nil {
		slog.Error("backfill", "user_id", userID, "ingested", n, "err", err)
		return
	}
	slog.Info("backfill complete", "user_id", userID, "ingested", n)
}

func (s *Service) upsertActivity(ctx context.Context, userID uuid.UUID, act strava.Activity) (uuid.UUID, error) {
	return s.q.UpsertActivity(ctx, database.UpsertActivityParams{
		UserID:     userID,
		Source:     strava.Source,
		SourceID:   strconv.FormatInt(act.ID, 10),
		StartTime:  act.StartTime,
		SportType:  act.SportType,
		RawSummary: act.Raw,
	})
}

func floats(xs []float64) []string {
	if len(xs) == 0 {
		return nil
	}
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = strconv.FormatFloat(x, 'f', -1, 64)
	}
	return out
}
