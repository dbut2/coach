package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"naomi.run/coach"
	"naomi.run/database"
	"naomi.run/web"
)

const conversationWindow = 200

func (s *Service) conversation(c *gin.Context) {
	user := currentUser(c)
	debug := s.debugMode(c)

	rows, err := s.q.ListRecentMessages(c.Request.Context(), database.ListRecentMessagesParams{
		UserID: user.ID,
		Limit:  conversationWindow,
	})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if len(rows) == 0 {
		s.startOnboarding(user.ID)
	}

	msgs := make([]web.Message, 0, len(rows))
	for _, m := range rows {
		if m.Role == coach.RoleTool {
			if debug {
				msgs = append(msgs, web.Message{
					Role:    web.RoleTool,
					Tool:    m.ToolName.String,
					Content: toolArgs(m.ToolPayload),
					Time:    m.CreatedAt.In(s.loc).Format("3:04 PM"),
				})
			}
			continue
		}
		role := web.RoleUser
		if m.Role == coach.RoleCoach {
			role = web.RoleAssistant
		}
		msgs = append(msgs, web.Message{
			Role:    role,
			Content: m.Content,
			Time:    m.CreatedAt.In(s.loc).Format("3:04 PM"),
		})
	}
	render(c, http.StatusOK, web.Conversation(s.cfg.CoachName, msgs, s.pendingProposals(c, user.ID), debug))
}

const debugCookie = "debug"

func (s *Service) debugMode(c *gin.Context) bool {
	switch c.Query("debug") {
	case "1":
		c.SetCookie(debugCookie, "1", 0, "/", "", false, true)
		return true
	case "0":
		c.SetCookie(debugCookie, "", -1, "/", "", false, true)
		return false
	}
	v, _ := c.Cookie(debugCookie)
	return v == "1"
}

func toolArgs(p pqtype.NullRawMessage) string {
	if !p.Valid || len(p.RawMessage) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, p.RawMessage); err != nil {
		return string(p.RawMessage)
	}
	return buf.String()
}

func (s *Service) sendMessage(c *gin.Context) {
	user := currentUser(c)
	text := strings.TrimSpace(c.PostForm("message"))
	if text == "" {
		c.Status(http.StatusNoContent)
		return
	}

	go func() {
		uid := user.ID.String()
		s.hub.broadcast(uid, "typing", renderHTML(web.Typing()))
		defer s.hub.broadcast(uid, "typing", "")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if _, err := s.coach.Reply(ctx, uid, s.loc, text); err != nil {
			slog.Error("coach reply", "user", uid, "error", err)
		}
	}()

	render(c, http.StatusOK, web.Fragment(web.Message{
		Role:    web.RoleUser,
		Content: text,
		Time:    time.Now().In(s.loc).Format("3:04 PM"),
	}))
}

func (s *Service) startOnboarding(userID uuid.UUID) {
	if _, busy := s.onboarding.LoadOrStore(userID, struct{}{}); busy {
		return
	}

	go func() {
		defer s.onboarding.Delete(userID)

		uid := userID.String()
		s.hub.broadcast(uid, "typing", renderHTML(web.Typing()))
		defer s.hub.broadcast(uid, "typing", "")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if _, err := s.coach.Open(ctx, uid, s.loc); err != nil {
			slog.Error("coach open", "user", uid, "error", err)
		}
	}()
}

func (s *Service) conversationEvents(c *gin.Context) {
	user := currentUser(c)
	ch, cancel := s.hub.subscribe(user.ID.String())
	defer cancel()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		case ev := <-ch:
			_, _ = fmt.Fprintf(c.Writer, "event: %s\n", ev.name)
			for _, line := range strings.Split(ev.html, "\n") {
				_, _ = fmt.Fprintf(c.Writer, "data: %s\n", line)
			}
			_, _ = fmt.Fprint(c.Writer, "\n")
			c.Writer.Flush()
		}
	}
}

func (s *Service) settings(c *gin.Context) {
	user := currentUser(c)
	_, gErr := s.q.GetGarminConnection(c.Request.Context(), user.ID)
	render(c, http.StatusOK, web.Settings(web.SettingsData{
		DisplayName:     user.DisplayName.String,
		StravaConnected: true,
		GarminConnected: gErr == nil,
		GarminState:     c.Query("garmin"),
	}))
}
