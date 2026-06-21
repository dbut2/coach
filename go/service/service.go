package service

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"dbut.dev/web-template/go/database"
)

type Service struct {
	db database.Querier
	e  *gin.Engine
}

func New(db database.Querier) *Service {
	gin.SetMode(gin.ReleaseMode)

	s := &Service{
		db: db,
		e:  gin.New(),
	}

	s.addRoutes()

	return s
}

func (s *Service) addRoutes() {
	s.e.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.e.GET("/", s.rootHandler)

	v1 := s.e.Group("/api/v1")
	v1api := v1API{s: s}
	{
		v1.GET("/pageviews", v1api.getPageViews)
	}
}

func (s *Service) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    ":8080",
		Handler: s.e,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return err
}
