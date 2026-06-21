package service

import (
	"bytes"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"dbut.dev/web-template/web"
)

func (s *Service) rootHandler(c *gin.Context) {
	err := s.db.IncrementPageViews(c)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	views, err := s.db.GetPageViews(c)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	handlePage(c, http.StatusOK, web.Index(int(views)))
}

func handlePage(c *gin.Context, status int, component templ.Component) {
	buf := &bytes.Buffer{}
	err := component.Render(c, buf)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(status, "text/html", buf.Bytes())
}
