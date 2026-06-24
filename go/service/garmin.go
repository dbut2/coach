package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"naomi.run/database"
	"naomi.run/garmin"
	"naomi.run/web"
)

const (
	garminSource         = "garmin"
	wellnessBackfillDays = 30
	wellnessRefreshEvery = 3 * time.Hour
)

func (s *Service) garminToken(ctx context.Context, userID uuid.UUID) (bearer, displayName string, err error) {
	conn, err := s.q.GetGarminConnection(ctx, userID)
	if err != nil {
		return "", "", err
	}
	s.gmu.Lock()
	defer s.gmu.Unlock()
	if tok := s.gtok[userID]; tok != nil && time.Until(tok.ExpiresAt) > time.Minute {
		return tok.AccessToken, conn.DisplayName, nil
	}
	tok, err := s.garmin.Exchange(ctx, conn.OauthToken, conn.OauthSecret)
	if err != nil {
		return "", "", err
	}
	s.gtok[userID] = tok
	return tok.AccessToken, conn.DisplayName, nil
}

func (s *Service) connectGarmin(c *gin.Context) {
	user := currentUser(c)
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	if email == "" || password == "" {
		c.Redirect(http.StatusFound, "/settings?garmin=error")
		return
	}
	flow := s.garmin.Login(email, password)
	tok, err := flow.Start(c.Request.Context())
	if errors.Is(err, garmin.ErrMFARequired) {
		s.gmu.Lock()
		s.gpending[user.ID] = flow
		s.gmu.Unlock()
		c.Redirect(http.StatusFound, "/settings?garmin=mfa")
		return
	}
	if err != nil {
		log.Printf("garmin connect: %v", err)
		c.Redirect(http.StatusFound, "/settings?garmin=error")
		return
	}
	s.finishGarminConnect(c, user.ID, tok)
}

func (s *Service) garminMFA(c *gin.Context) {
	user := currentUser(c)
	code := strings.TrimSpace(c.PostForm("code"))
	s.gmu.Lock()
	flow := s.gpending[user.ID]
	s.gmu.Unlock()
	if flow == nil || code == "" {
		c.Redirect(http.StatusFound, "/settings?garmin=error")
		return
	}
	tok, err := flow.SubmitMFA(c.Request.Context(), code)
	if err != nil {
		log.Printf("garmin mfa: %v", err)
		c.Redirect(http.StatusFound, "/settings?garmin=mfa")
		return
	}
	s.finishGarminConnect(c, user.ID, tok)
}

func (s *Service) finishGarminConnect(c *gin.Context, userID uuid.UUID, tok *garmin.Tokens) {
	if _, err := s.q.UpsertGarminConnection(c.Request.Context(), database.UpsertGarminConnectionParams{
		UserID:      userID,
		OauthToken:  tok.OAuth1Token,
		OauthSecret: tok.OAuth1Secret,
		DisplayName: tok.DisplayName,
		FullName:    tok.FullName,
	}); err != nil {
		log.Printf("store garmin connection: %v", err)
		c.Redirect(http.StatusFound, "/settings?garmin=error")
		return
	}
	s.gmu.Lock()
	s.gtok[userID] = &tok.Access
	delete(s.gpending, userID)
	s.gmu.Unlock()
	go s.garminBackfill(userID)
	c.Redirect(http.StatusFound, "/settings?garmin=ok")
}

func (s *Service) disconnectGarmin(c *gin.Context) {
	user := currentUser(c)
	if err := s.q.DeleteGarminConnection(c.Request.Context(), user.ID); err != nil {
		render(c, http.StatusInternalServerError, web.ErrorPage(s.cfg.CoachName, "We couldn't disconnect Garmin. Please try again."))
		return
	}
	s.gmu.Lock()
	delete(s.gtok, user.ID)
	delete(s.gpending, user.ID)
	s.gmu.Unlock()
	c.Redirect(http.StatusFound, "/settings")
}

func (s *Service) syncGarminNow(c *gin.Context) {
	user := currentUser(c)
	if _, err := s.q.GetGarminConnection(c.Request.Context(), user.ID); err != nil {
		c.Redirect(http.StatusFound, "/settings")
		return
	}
	go s.garminBackfill(user.ID)
	c.Redirect(http.StatusFound, "/settings?garmin=ok")
}

func (s *Service) garminBackfill(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bearer, displayName, err := s.garminToken(ctx, userID)
	if err != nil {
		log.Printf("garmin backfill token: %v", err)
		return
	}
	n := s.syncWellness(ctx, userID, bearer, displayName, wellnessBackfillDays)
	s.markSynced(ctx, userID)
	log.Printf("garmin backfill user %s: %d wellness days", userID, n)
}

func (s *Service) wellnessLoop(ctx context.Context) {
	t := time.NewTicker(wellnessRefreshEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refreshAllWellness(ctx)
		}
	}
}

func (s *Service) refreshAllWellness(ctx context.Context) {
	conns, err := s.q.ListGarminConnections(ctx)
	if err != nil {
		log.Printf("garmin refresh list: %v", err)
		return
	}
	for _, conn := range conns {
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		bearer, displayName, err := s.garminToken(cctx, conn.UserID)
		if err != nil {
			log.Printf("garmin refresh token %s: %v", conn.UserID, err)
			cancel()
			continue
		}
		if n := s.syncWellness(cctx, conn.UserID, bearer, displayName, 2); n > 0 {
			s.markSynced(cctx, conn.UserID)
		}
		cancel()
	}
}

func (s *Service) syncWellness(ctx context.Context, userID uuid.UUID, bearer, displayName string, days int) int {
	today := s.today()
	stored := 0
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, -i)
		w, err := s.garmin.Wellness(ctx, bearer, displayName, d.Format("2006-01-02"))
		if err != nil {
			log.Printf("garmin wellness %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		if !w.HasData() {
			continue
		}
		if err := s.storeWellness(ctx, userID, d, w); err != nil {
			log.Printf("store wellness %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		stored++
	}
	return stored
}

func (s *Service) storeWellness(ctx context.Context, userID uuid.UUID, day time.Time, w *garmin.Wellness) error {
	var sleep *int
	if w.SleepSeconds > 0 {
		m := w.SleepMinutes()
		sleep = &m
	}
	nums := []struct {
		key string
		val *int
	}{
		{"hrv", w.HRVLastNightAvg},
		{"resting_hr", w.RestingHR},
		{"sleep_minutes", sleep},
		{"stress_level", w.AvgStress},
		{"body_battery", w.BodyBatteryHigh},
		{"readiness", w.ReadinessScore},
	}
	for _, n := range nums {
		if n.val == nil {
			continue
		}
		if err := s.q.UpsertWellnessMetric(ctx, database.UpsertWellnessMetricParams{
			UserID:    userID,
			Date:      day,
			MetricKey: n.key,
			ValueNum:  sql.NullString{String: strconv.Itoa(*n.val), Valid: true},
			Source:    garminSource,
		}); err != nil {
			return err
		}
	}
	if len(w.Raw) > 0 {
		if err := s.q.UpsertWellnessMetric(ctx, database.UpsertWellnessMetricParams{
			UserID:    userID,
			Date:      day,
			MetricKey: "garmin_raw",
			ValueJson: pqtype.NullRawMessage{RawMessage: w.Raw, Valid: true},
			Source:    garminSource,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) markSynced(ctx context.Context, userID uuid.UUID) {
	_ = s.q.UpdateGarminLastSync(ctx, database.UpdateGarminLastSyncParams{
		UserID:   userID,
		LastSync: sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Service) today() time.Time {
	now := time.Now().In(s.loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
