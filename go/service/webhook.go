package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"naomi.run/database"
	"naomi.run/strava"
)

func (s *Service) webhookVerify(c *gin.Context) {
	if c.Query("hub.mode") != "subscribe" || c.Query("hub.verify_token") != s.cfg.WebhookVerifyToken {
		c.Status(http.StatusForbidden)
		return
	}
	c.JSON(http.StatusOK, gin.H{"hub.challenge": c.Query("hub.challenge")})
}

type webhookEvent struct {
	ObjectType string `json:"object_type"`
	ObjectID   int64  `json:"object_id"`
	AspectType string `json:"aspect_type"`
	OwnerID    int64  `json:"owner_id"`
}

func (s *Service) webhookEvent(c *gin.Context) {
	var ev webhookEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusOK)

	if ev.ObjectType != "activity" {
		return
	}
	go s.handleActivityEvent(ev)
}

func (s *Service) handleActivityEvent(ev webhookEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := s.q.GetStravaConnectionByAthleteID(ctx, ev.OwnerID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("webhook: load connection", "athlete_id", ev.OwnerID, "err", err)
		}
		return
	}

	switch ev.AspectType {
	case "create", "update":
		if err := s.ingestActivity(ctx, conn, ev.ObjectID); err != nil {
			slog.Error("webhook: ingest", "activity_id", ev.ObjectID, "err", err)
		}
	case "delete":
		if err := s.q.DeleteActivity(ctx, database.DeleteActivityParams{
			Source:   strava.Source,
			SourceID: strconv.FormatInt(ev.ObjectID, 10),
		}); err != nil {
			slog.Error("webhook: delete", "activity_id", ev.ObjectID, "err", err)
		}
	}
}
