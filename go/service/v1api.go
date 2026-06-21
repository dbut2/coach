package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type v1API struct {
	s *Service
}

func (a *v1API) getPageViews(c *gin.Context) {
	views, err := a.s.db.GetPageViews(c)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"pageviews": views})
}
