package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"naomi.run/coach"
	"naomi.run/database"
	"naomi.run/web"
)

const conversationWindow = 200

func (s *Service) conversation(c *gin.Context) {
	user := currentUser(c)
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
	render(c, http.StatusOK, web.Conversation(s.cfg.CoachName, msgs, s.pendingProposals(c, user.ID)))
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
