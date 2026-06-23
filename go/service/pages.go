package service

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"naomi.run/web"
)

func (s *Service) conversation(c *gin.Context) {
	render(c, http.StatusOK, web.Conversation(s.cfg.CoachName, nil))
}

func (s *Service) settings(c *gin.Context) {
	user := currentUser(c)
	render(c, http.StatusOK, web.Settings(web.SettingsData{
		DisplayName:     user.DisplayName.String,
		StravaConnected: true,
	}))
}
