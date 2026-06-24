package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
		c.Redirect(http.StatusSeeOther, "/conversation")
		return
	}
	if _, err := s.coach.Reply(c.Request.Context(), user.ID.String(), s.loc, text); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/conversation")
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
